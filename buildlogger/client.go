package buildlogger

import (
	"context"
	"crypto/tls"
	"strings"
	"sync"
	"time"

	"github.com/evergreen-ci/aviation"
	"github.com/evergreen-ci/timber/buildlogger/internal"
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
	mu         sync.Mutex
	ctx        context.Context
	opts       *LoggerOptions
	conn       *grpc.ClientConn
	client     internal.BuildloggerClient
	buffer     []*internal.LogLine
	bufferSize int
	*send.Base
}

// LoggerOptions support the use and creation of a Buildlogger log.
type LoggerOptions struct {
	// Unique information to identify the log.
	Project          string
	Version          string
	Variant          string
	TaskName         string
	TaskID           string
	Execution        int32
	TestName         string
	Trial            int32
	ProcessName      string
	LogFormatText    bool
	LogFormatJSON    bool
	LogFormatBSON    bool
	LogStorageS3     bool
	LogStorageLocal  bool
	LogStorageGridFS bool
	Arguments        map[string]string
	Mainline         bool

	// Configure a local sender for "fallback" operations and to collect
	// the location of the buildlogger output.
	Local send.Sender

	// The number max number of bytes to buffer before sending log data
	// over rpc to Cedar.
	MaxBufferSize int

	// Turn checking for new lines in messages off. If this is set to true
	// make sure log messages do not contain new lines, otherwise the logs
	// will be stored incorrectly.
	NewLineCheckOff bool

	// The gRPC client connection. If nil, a new connection will be
	// established with the gRPC connection configuration.
	ClientConn *grpc.ClientConn

	// Configuration for gRPC client connection.
	RPCAddress string
	Insecure   bool
	CAFile     string
	CertFile   string
	KeyFile    string

	logID    string
	format   internal.LogFormat
	storage  internal.LogStorage
	exitCode int32
}

func (opts *LoggerOptions) validate() error {
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

	count = 0
	opts.storage = internal.LogStorage_LOG_STORAGE_S3
	if opts.LogStorageS3 {
		count++
	}
	if opts.LogStorageLocal {
		opts.storage = internal.LogStorage_LOG_STORAGE_LOCAL
		count++
	}
	if opts.LogStorageGridFS {
		opts.storage = internal.LogStorage_LOG_STORAGE_GRIDFS
		count++
	}
	if count > 1 {
		return errors.New("cannot specify more than one storage type")
	}

	if opts.ClientConn == nil {
		if opts.RPCAddress == "" {
			return errors.New("must specify a RPC address when a client connection is not provided")
		}
		if !opts.Insecure && (opts.CAFile == "" || opts.CertFile == "" || opts.KeyFile == "") {
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

// SetExitCode sets the exit code variable.
func (opts *LoggerOptions) SetExitCode(i int32) { opts.exitCode = i }

// GetLogID returns the unique buildlogger log ID set after NewLogger is
// called.
func (opts *LoggerOptions) GetLogID() string {
	return opts.logID
}

// NewLogger returns a grip Sender backed by Cedar Buildlogger with level
// information set.
func NewLogger(ctx context.Context, name string, l send.LevelInfo, opts *LoggerOptions) (send.Sender, error) {
	b, err := MakeLogger(ctx, name, opts)
	if err != nil {
		return nil, errors.Wrap(err, "problem making new logger")
	}

	if err := b.SetLevel(l); err != nil {
		return nil, errors.Wrap(err, "problem setting grip level")
	}

	return b, nil
}

// MakeLogger returns a grip Sender backed by Cedar Buildlogger.
func MakeLogger(ctx context.Context, name string, opts *LoggerOptions) (send.Sender, error) {
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

	if err := b.createNewLog(ts); err != nil {
		return nil, err
	}

	return b, nil
}

// Send sends the given message with a timestamp created when the function is
// called to the Cedar Buildlogger backend. This function buffers the messages
// until the maximum allowed buffer size is reached, at which point the
// messages in the buffer are sent to the Buildlogger server via RPC. Send is
// thread safe.
func (b *buildlogger) Send(m message.Composer) {
	if !b.Level().ShouldLog(m) {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	ts := time.Now()

	_, ok := m.(*message.GroupComposer)
	var lines []string
	if b.opts.NewLineCheckOff && !ok {
		lines = []string{m.String()}
	} else {
		lines = strings.Split(m.String(), "\n")
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		logLine := &internal.LogLine{
			Timestamp: &timestamp.Timestamp{Seconds: ts.Unix(), Nanos: int32(ts.Nanosecond())},
			Data:      line,
		}

		if b.bufferSize+len(logLine.Data) > b.opts.MaxBufferSize {
			if err := b.flush(); err != nil {
				b.opts.Local.Send(message.NewErrorMessage(level.Error, err))
				return
			}
		}

		b.buffer = append(b.buffer, logLine)
		b.bufferSize += len(logLine.Data)
	}
}

// Close flushes anything that may be left in the underlying buffer and closes
// out the log with a completed at timestamp and the exit code. If the gRPC
// client connection was created in NewLogger or MakeLogger, this connection is
// also closed. Close is thread safe but should only be called once no more
// calls to Send are needed; after Close has been called any subsequent calls
// to Send will error.
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
			ExitCode:    b.opts.exitCode,
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

func (b *buildlogger) createNewLog(ts time.Time) error {
	data := &internal.LogData{
		Info: &internal.LogInfo{
			Project:   b.opts.Project,
			Version:   b.opts.Version,
			Variant:   b.opts.Variant,
			TaskName:  b.opts.TaskName,
			TaskId:    b.opts.TaskID,
			Execution: b.opts.Execution,
			TestName:  b.opts.TestName,
			Trial:     b.opts.Trial,
			ProcName:  b.opts.ProcessName,
			Format:    b.opts.format,
			Arguments: b.opts.Arguments,
			Mainline:  b.opts.Mainline,
		},
		Storage:   b.opts.storage,
		CreatedAt: &timestamp.Timestamp{Seconds: ts.Unix(), Nanos: int32(ts.Nanosecond())},
	}
	resp, err := b.client.CreateLog(b.ctx, data)
	if err != nil {
		b.opts.Local.Send(message.NewErrorMessage(level.Error, err))
		return errors.Wrap(err, "problem creating log")
	}
	b.opts.logID = resp.LogId

	return nil
}

func (b *buildlogger) flush() error {
	_, err := b.client.AppendLogLines(b.ctx, &internal.LogLines{
		LogId: b.opts.logID,
		Lines: b.buffer,
	})
	if err != nil {
		return err
	}

	b.buffer = []*internal.LogLine{}
	b.bufferSize = 0

	return nil
}
