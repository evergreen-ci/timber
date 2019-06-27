package buildlogger

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"time"

	"github.com/evergreen-ci/aviation"
	"github.com/evergreen-ci/timber/rpc/internal"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type buildlogger struct {
	ctx      context.Context
	opts     *BuildloggerOptions
	conn     *grpc.ClientConn
	client   internal.BuildloggerClient
	buffer   []*internal.LogLine
	exitCode int32
	*send.Base
}

type BuildloggerOptions struct {
	// Unique information to identify the log.
	Project       string
	Version       string
	Variant       string
	TaskName      string
	TaskID        string
	Execution     int32
	TestName      string
	Trial         int32
	ProcessName   string
	LogFormatJSON bool
	LogFormatBSON bool
	LogFormatText bool
	Arguments     map[string]string
	Mainline      bool

	// Configure a local sender for "fallback" operations and to collect
	// the location of the buildlogger output.
	Local send.Sender

	// The number max number of bytes to buffer before sending log data
	// over rpc to Cedar.
	MaxBufferSize int

	// The gRPC client connection. If nil, a new connection will be
	// established with the gRPC connection configuration.
	ClientConn *grpc.ClientConn

	// Configuration for gRPC client connection.
	RPCAddress string
	Insecure   bool
	CAFile     string
	CertFile   string
	KeyFile    string

	logID       string
	format      internal.LogFormat
	exitCode    int32
	exitCodeSet bool
}

func (opts *BuildloggerOptions) validate() error {
	count := 0
	if opts.LogFormatText {
		opts.format = internal.LogFormat_LOG_FORMAT_TEXT
		count++
	}
	if opts.LogFormatJSON {
		opts.format = internal.LogFormat_LOG_FORMAT_JSON
		count++
	}
	if opts.LogFormatBSON {
		opts.format = internal.LogFormat_LOG_FORMAT_BSON
		count++
	}
	if count > 1 {
		return errors.New("cannot specify more than one log format")
	}

	if opts.ClientConn == nil {
		if opts.RPCAddress == "" {
			return errors.New("must specify a RPC address when a client connection is not provided")
		}
		if opts.Insecure && (opts.CAFile == "" || opts.CertFile == "" || opts.KeyFile == "") {
			return errors.New("must specify credential files when making a secure connection over RPC")
		}
	}

	if opts.Local == nil {
		opts.Local = send.MakeNative()
	}

	if opts.MaxBufferSize == 0 {
		// TODO: figure out ideal default size
		opts.MaxBufferSize = 4096
	}

	return nil
}

func (opts *BuildloggerOptions) GetLogID() string {
	return opts.logID
}

func (opts *BuildloggerOptions) SetExitCode(i int32) error {
	if opts.exitcodeSet {
		return errors.New("exit code has already been set for this buildlogger log")
	}

	opts.exitCode = i
	opts.exitCodeSet = true
}

func NewBuildlogger(ctx context.Context, name string, l send.LevelInfo, opts BuildloggerOptions) (send.Sender, error) {
	ts := time.Now()

	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "invalid cedar buildlogger options")
	}

	var conn *grpc.ClientConn
	var err error
	if opts.ClientConn == nil {
		rpcOpts := []grpc.DialOption{
			grpc.WithUnaryInterceptor(aviation.MakeRetryUnaryClientInterceptor(10)),
			grpc.WithStreamInterceptor(aviation.MakeRetryStreamClientInterceptor(10)),
		}
		if opts.Insecure {
			rpcOpts = append(rpcOpts, grpc.WithInsecure())
		} else {
			var tlsConf *tls.Config
			tlsConf, err = aviation.GetClientTLSConfig(opts.CAFile, opts.CertFile, opts.KeyFile)
			if err != nil {
				return nil, errors.Wrap(err, "problem getting client TLS config")
			}

			rpcOpts = append(rpcOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConf)))
		}

		conn, err := grpc.DialContext(ctx, opts.RPCAddress, rpcOpts...)
		if err != nil {
			return nil, errors.Wrap(err, "problem dialing rpc server")
		}
		opts.ClientConn = conn
	}

	b := &buildlogger{
		ctx:    ctx,
		opts:   opts,
		conn:   conn,
		client: internal.NewBuildloggerClient(opts.ClientConn),
		buffer: []*internal.LogLine{},
		Base:   send.NewBase(name),
	}

	if err := b.SetErrorHandler(send.ErrorHandlerFromSender(b.opts.Local)); err != nil {
		return nil, errors.Wrap(err, "problem setting default error handler")
	}

	data := &internal.LogData{
		Info: &internal.LogInfo{
			Project:   opts.Project,
			Version:   opts.Version,
			Variant:   opts.Variant,
			TaskName:  opts.TaskName,
			TaskId:    opts.TaskID,
			Execution: opts.Execution,
			TestName:  opts.TestName,
			Trial:     opts.Trial,
			ProcName:  opts.ProcessName,
			Format:    opts.format,
			Arguments: opts.Arguments,
			Mainline:  opts.Mainline,
		},
		Storage:   internal.LogStorage_LOG_STORAGE_S3,
		CreatedAt: &timestamp.Timestamp{Seconds: ts.Unix(), Nanos: int32(ts.Nanosecond())},
	}
	resp, err := b.client.CreateLog(ctx, data)
	if err != nil {
		b.opts.Local.Send(message.NewErrorMessage(level.Error, err))
		return nil, errors.Wrap(err, "problem creating log")
	}
	b.opts.logID = resp.LogId

	return b, nil
}

func (b *buildlogger) Send(m message.Composer) {
	ts := time.Now()

	if b.Level().ShouldLog(m) {
		logLine := &internal.LogLine{
			Timestamp: &timestamp.Timestamp{Seconds: ts.Unix(), Nanos: int32(ts.Nanosecond())},
			Data:      m.String(),
		}

		if binary.Size(b.buffer)+binary.Size(logLine) > b.maxBufferSize {
			if err := b.flush(); err != nil {
				b.opts.Local.Send(message.NewErrorMessage(level.Error, err))
				return
			}
		}

		b.buffer = append(b.buffer, logLine)
	}
}

func (b *buildlogger) Close() error {
	ts := time.Now()

	catcher := grip.NewBasicCatcher()

	if len(b.buffer) > 0 {
		if err := b.flush(); err != nil {
			b.opts.Local.Send(message.NewErrorMessage(level.Error, err))
			catcher.Add(errors.Wrap(err, "problem flushing buffer"))
		}
	}

	if !catcher.HasErrors() {
		endInfo := &internal.LogEndInfo{
			LogId:       b.opts.logID,
			ExitCode:    b.exitCode,
			CompletedAt: &timestamp.Timestamp{Seconds: ts.Unix(), Nanos: int32(ts.Nanosecond())},
		}
		_, err := b.client.CloseLog(b.ctx, endInfo)
		b.opts.Local.Send(message.NewErrorMessage(level.Error, err))
		catcher.Add(errors.Wrap(err, "problem closing log"))
	}

	if b.conn != nil {
		catcher.Add(b.conn.Close())
	}

	return catcher.Resolve()
}

func (b *buildlogger) flush() error {
	_, err := b.client.AppendLogLines(b.ctx, &internal.LogLines{
		LogId: b.opts.logID,
		Lines: b.buffer,
	})
	if err != nil {
		return errors.Wrap(err, "problem appending lines")
	}

	b.buffer = []*internal.LogLine{}

	return nil
}
