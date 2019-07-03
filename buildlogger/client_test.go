package buildlogger

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/evergreen-ci/timber/buildlogger/internal"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

var r = rand.New(rand.NewSource(time.Now().Unix()))

type mockClient struct {
	createErr  bool
	appendErr  bool
	closeErr   bool
	logData    *internal.LogData
	logLines   *internal.LogLines
	logEndInfo *internal.LogEndInfo
}

func (mc *mockClient) CreateLog(_ context.Context, in *internal.LogData, _ ...grpc.CallOption) (*internal.BuildloggerResponse, error) {
	if mc.createErr {
		return nil, errors.New("create error")
	}

	mc.logData = in

	return &internal.BuildloggerResponse{LogId: in.Info.TestName}, nil
}

func (mc *mockClient) AppendLogLines(_ context.Context, in *internal.LogLines, _ ...grpc.CallOption) (*internal.BuildloggerResponse, error) {
	if mc.appendErr {
		return nil, errors.New("append error")
	}

	mc.logLines = in

	return &internal.BuildloggerResponse{LogId: in.LogId}, nil
}

func (*mockClient) StreamLog(_ context.Context, _ ...grpc.CallOption) (internal.Buildlogger_StreamLogClient, error) {
	return nil, nil
}

func (mc *mockClient) CloseLog(_ context.Context, in *internal.LogEndInfo, _ ...grpc.CallOption) (*internal.BuildloggerResponse, error) {
	if mc.closeErr {
		return nil, errors.New("close error")
	}

	mc.logEndInfo = in

	return &internal.BuildloggerResponse{LogId: in.LogId}, nil
}

type mockService struct {
	createLog      bool
	appendLogLines bool
	closeLog       bool
	createErr      bool
	appendErr      bool
	closeErr       bool
}

func (ms *mockService) CreateLog(_ context.Context, in *internal.LogData) (*internal.BuildloggerResponse, error) {
	if ms.createErr {
		return nil, errors.New("create error")
	}

	ms.createLog = true
	return &internal.BuildloggerResponse{}, nil
}

func (ms *mockService) AppendLogLines(_ context.Context, in *internal.LogLines) (*internal.BuildloggerResponse, error) {
	if ms.appendErr {
		return nil, errors.New("append error")
	}

	ms.appendLogLines = true
	return &internal.BuildloggerResponse{}, nil
}

func (ms *mockService) StreamLog(_ internal.Buildlogger_StreamLogServer) error { return nil }

func (ms *mockService) CloseLog(_ context.Context, in *internal.LogEndInfo) (*internal.BuildloggerResponse, error) {
	if ms.closeErr {
		return nil, errors.New("close error")
	}

	ms.closeLog = true
	return &internal.BuildloggerResponse{}, nil
}

type mockSender struct {
	*send.Base
	lastMessage string
}

func (ms *mockSender) Send(m message.Composer) {
	if ms.Level().ShouldLog(m) {
		ms.lastMessage = m.String()
	}
}

func TestLoggerOptionsValidate(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		opts := &LoggerOptions{ClientConn: &grpc.ClientConn{}}
		require.NoError(t, opts.validate())
		assert.Equal(t, opts.format, internal.LogFormat_LOG_FORMAT_UNKNOWN)
		assert.Equal(t, opts.storage, internal.LogStorage_LOG_STORAGE_S3)
		assert.NotNil(t, opts.Local)
		assert.Equal(t, opts.MaxBufferSize, 4096)

		size := 100
		local := &mockSender{Base: send.NewBase("test")}
		opts = &LoggerOptions{
			ClientConn:    &grpc.ClientConn{},
			MaxBufferSize: size,
			Local:         local,
		}
		require.NoError(t, opts.validate())
		assert.Equal(t, size, opts.MaxBufferSize)
		assert.Equal(t, local, opts.Local)

		opts.LogFormatText = true
		require.NoError(t, opts.validate())
		assert.Equal(t, opts.format, internal.LogFormat_LOG_FORMAT_TEXT)
		opts.LogFormatText = false
		opts.LogFormatJSON = true
		require.NoError(t, opts.validate())
		assert.Equal(t, opts.format, internal.LogFormat_LOG_FORMAT_JSON)
		opts.LogFormatJSON = false
		opts.LogFormatBSON = true
		require.NoError(t, opts.validate())
		assert.Equal(t, opts.format, internal.LogFormat_LOG_FORMAT_BSON)

		opts.LogStorageS3 = true
		require.NoError(t, opts.validate())
		assert.Equal(t, opts.storage, internal.LogStorage_LOG_STORAGE_S3)
		opts.LogStorageS3 = false
		opts.LogStorageLocal = true
		require.NoError(t, opts.validate())
		assert.Equal(t, opts.storage, internal.LogStorage_LOG_STORAGE_LOCAL)
		opts.LogStorageLocal = false
		opts.LogStorageGridFS = true
		require.NoError(t, opts.validate())
		assert.Equal(t, opts.storage, internal.LogStorage_LOG_STORAGE_GRIDFS)
	})
	t.Run("MultipleLogFormats", func(t *testing.T) {
		opts := &LoggerOptions{
			ClientConn:    &grpc.ClientConn{},
			LogFormatText: true,
			LogFormatJSON: true,
			LogFormatBSON: true,
		}
		assert.Error(t, opts.validate())
	})
	t.Run("MultipleStorage", func(t *testing.T) {
		opts := &LoggerOptions{
			ClientConn:       &grpc.ClientConn{},
			LogStorageS3:     true,
			LogStorageLocal:  true,
			LogStorageGridFS: true,
		}
		assert.Error(t, opts.validate())
	})
	t.Run("ClientConnNilAndNoRPCAddress", func(t *testing.T) {
		opts := &LoggerOptions{
			Insecure: true,
		}
		assert.Error(t, opts.validate())
	})
	t.Run("InsecureFalseAndNoAuthFiles", func(t *testing.T) {
		opts := &LoggerOptions{
			RPCAddress: "address",
		}
		assert.Error(t, opts.validate())
	})
}

func TestNewLogger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := &mockService{}
	require.NoError(t, startRPCService(ctx, srv, 4000))
	addr := fmt.Sprintf("localhost:%d", 4000)
	conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure())
	require.NoError(t, err)

	t.Run("WithExistingClient", func(t *testing.T) {
		name := "test"
		l := send.LevelInfo{Default: level.Debug, Threshold: level.Debug}
		opts := &LoggerOptions{
			ClientConn: conn,
			Local:      &mockSender{Base: send.NewBase("test")},
		}

		s, err := NewLogger(ctx, name, l, opts)
		require.NoError(t, err)
		require.NotNil(t, s)
		assert.Equal(t, name, s.Name())
		assert.Equal(t, l, s.Level())
		assert.True(t, srv.createLog)
		b, ok := s.(*buildlogger)
		require.True(t, ok)
		assert.Equal(t, ctx, b.ctx)
		assert.NotNil(t, b.buffer)
		assert.Equal(t, opts, b.opts)
		srv.createLog = false
	})
	t.Run("WithoutExistingClient", func(t *testing.T) {
		name := "test2"
		l := send.LevelInfo{Default: level.Trace, Threshold: level.Alert}
		opts := &LoggerOptions{
			Local:      &mockSender{Base: send.NewBase("test")},
			Insecure:   true,
			RPCAddress: addr,
		}

		s, err := NewLogger(ctx, name, l, opts)
		require.NoError(t, err)
		require.NotNil(t, s)
		assert.Equal(t, name, s.Name())
		assert.Equal(t, l, s.Level())
		assert.True(t, srv.createLog)
		b, ok := s.(*buildlogger)
		require.True(t, ok)
		assert.Equal(t, ctx, b.ctx)
		assert.NotNil(t, b.buffer)
		assert.Equal(t, opts, b.opts)
		srv.createLog = false
	})
	t.Run("InvalidOptions", func(t *testing.T) {
		s, err := NewLogger(ctx, "test3", send.LevelInfo{}, &LoggerOptions{})
		assert.Error(t, err)
		assert.Nil(t, s)
	})
	t.Run("CreateLogError", func(t *testing.T) {
		name := "test4"
		l := send.LevelInfo{Default: level.Debug, Threshold: level.Debug}
		ms := &mockSender{Base: send.NewBase("test")}
		opts := &LoggerOptions{
			ClientConn: conn,
			Local:      ms,
		}
		srv.createErr = true

		s, err := NewLogger(ctx, name, l, opts)
		assert.Error(t, err)
		assert.Nil(t, s)
		assert.True(t, strings.Contains(ms.lastMessage, "create error"))
	})
}

func TestCreateNewLog(t *testing.T) {
	t.Run("CorrectData", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		ts := time.Now()
		expectedTS := &timestamp.Timestamp{Seconds: ts.Unix(), Nanos: int32(ts.Nanosecond())}

		require.NoError(t, b.createNewLog(ts))
		assert.Equal(t, b.opts.Project, mc.logData.Info.Project)
		assert.Equal(t, b.opts.Version, mc.logData.Info.Version)
		assert.Equal(t, b.opts.Variant, mc.logData.Info.Variant)
		assert.Equal(t, b.opts.TaskName, mc.logData.Info.TaskName)
		assert.Equal(t, b.opts.Execution, mc.logData.Info.Execution)
		assert.Equal(t, b.opts.TestName, mc.logData.Info.TestName)
		assert.Equal(t, b.opts.Trial, mc.logData.Info.Trial)
		assert.Equal(t, b.opts.ProcessName, mc.logData.Info.ProcName)
		assert.Equal(t, b.opts.format, mc.logData.Info.Format)
		assert.Equal(t, b.opts.Arguments, mc.logData.Info.Arguments)
		assert.Equal(t, b.opts.Mainline, mc.logData.Info.Mainline)
		assert.Equal(t, expectedTS, mc.logData.CreatedAt)
		assert.Equal(t, internal.LogStorage_LOG_STORAGE_S3, mc.logData.Storage)
		assert.Equal(t, b.opts.logID, mc.logData.Info.TestName)
		assert.Empty(t, ms.lastMessage)
	})
	t.Run("RPCError", func(t *testing.T) {
		mc := &mockClient{createErr: true}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)

		assert.Error(t, b.createNewLog(time.Now()))
		assert.Equal(t, "create error", ms.lastMessage)
	})
}

func TestSend(t *testing.T) {
	t.Run("RespectsPriority", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)

		require.NoError(t, b.SetLevel(send.LevelInfo{Default: level.Debug, Threshold: level.Emergency}))
		m := message.ConvertToComposer(level.Alert, "alert")
		b.Send(m)
		assert.Empty(t, b.buffer)
		m = message.ConvertToComposer(level.Emergency, "emergency")
		b.Send(m)
		require.NotEmpty(t, b.buffer)
		assert.Equal(t, m.String(), b.buffer[len(b.buffer)-1].Data)

		require.NoError(t, b.SetLevel(send.LevelInfo{Default: level.Debug, Threshold: level.Debug}))
		m = message.ConvertToComposer(level.Debug, "debug")
		b.Send(m)
		require.NotEmpty(t, b.buffer)
		assert.Equal(t, m.String(), b.buffer[len(b.buffer)-1].Data)
	})
	t.Run("FlushAtCapacityWithNewLineCheck", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.logID = "id"
		b.opts.MaxBufferSize = 4096
		size := 256
		messages := []string{}

		for b.bufferSize < b.opts.MaxBufferSize {
			if b.bufferSize-size <= 0 {
				size = b.opts.MaxBufferSize - b.bufferSize
			}

			m := message.ConvertToComposer(level.Debug, newRandString(size))
			b.Send(m)
			require.Empty(t, ms.lastMessage)

			lines := strings.Split(m.String(), "\n")
			for i, line := range lines {
				if len(line) == 0 {
					continue
				}
				assert.Nil(t, mc.logLines)
				assert.Equal(t, time.Now().Unix(), b.buffer[len(b.buffer)-1].Timestamp.Seconds)
				require.True(t, len(b.buffer) >= len(lines))
				assert.Equal(t, line, b.buffer[len(b.buffer)-(len(lines)-i)].Data)
				messages = append(messages, line)
			}
		}
		assert.Equal(t, b.opts.MaxBufferSize, b.bufferSize)
		m := message.ConvertToComposer(level.Debug, "overflow")
		b.Send(m)
		require.Len(t, b.buffer, 1)
		assert.Equal(t, time.Now().Unix(), b.buffer[0].Timestamp.Seconds)
		assert.Equal(t, m.String(), b.buffer[0].Data)
		assert.Equal(t, len(m.String()), b.bufferSize)
		require.NotNil(t, mc.logLines)
		assert.Equal(t, b.opts.logID, mc.logLines.LogId)
		assert.Len(t, mc.logLines.Lines, len(messages))
		for i := range mc.logLines.Lines {
			assert.Equal(t, messages[i], mc.logLines.Lines[i].Data)
		}
	})
	t.Run("FlushAtCapacityWithOutNewLineCheck", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.logID = "id"
		b.opts.MaxBufferSize = 4096
		b.opts.NewLineCheckOff = true
		size := 256
		messages := []message.Composer{}

		for b.bufferSize < b.opts.MaxBufferSize {
			m := message.ConvertToComposer(level.Debug, newRandString(size))
			b.Send(m)
			require.Empty(t, ms.lastMessage)

			assert.Nil(t, mc.logLines)
			assert.Equal(t, time.Now().Unix(), b.buffer[len(b.buffer)-1].Timestamp.Seconds)
			assert.NotEmpty(t, b.buffer)
			assert.Equal(t, m.String(), b.buffer[len(b.buffer)-1].Data)
			messages = append(messages, m)
		}
		assert.Equal(t, b.opts.MaxBufferSize, b.bufferSize)
		m := message.ConvertToComposer(level.Debug, "overflow")
		b.Send(m)
		require.Len(t, b.buffer, 1)
		assert.Equal(t, time.Now().Unix(), b.buffer[0].Timestamp.Seconds)
		assert.Equal(t, m.String(), b.buffer[0].Data)
		assert.Equal(t, len(m.String()), b.bufferSize)
		require.NotEmpty(t, mc.logLines)
		assert.Equal(t, b.opts.logID, mc.logLines.LogId)
		assert.Len(t, mc.logLines.Lines, len(messages))
		for i := range mc.logLines.Lines {
			assert.Equal(t, messages[i].String(), mc.logLines.Lines[i].Data)
		}
	})
	t.Run("GroupComposer", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.MaxBufferSize = 4096
		b.opts.NewLineCheckOff = true

		m1 := message.ConvertToComposer(level.Debug, "Hello world!\nThis has a new line.")
		m2 := message.ConvertToComposer(level.Debug, "Goodbye world!")
		m := message.NewGroupComposer([]message.Composer{m1, m2})
		b.Send(m)
		assert.Len(t, b.buffer, 3)
		assert.Equal(t, len(m1.String())+len(m2.String())-1, b.bufferSize)
		assert.Equal(t, strings.Split(m1.String(), "\n")[0], b.buffer[0].Data)
		assert.Equal(t, strings.Split(m1.String(), "\n")[1], b.buffer[1].Data)
		assert.Equal(t, m2.String(), b.buffer[2].Data)
	})
	t.Run("RPCError", func(t *testing.T) {
		str := "overflow"
		mc := &mockClient{appendErr: true}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.MaxBufferSize = len(str)

		m1 := message.ConvertToComposer(level.Debug, str)
		m2 := message.ConvertToComposer(level.Debug, str)
		b.Send(m1)
		b.Send(m2)
		assert.Len(t, b.buffer, 1)
		assert.Equal(t, len(m2.String()), b.bufferSize)
		assert.Equal(t, "append error", ms.lastMessage)
	})
	t.Run("ClosedSender", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.logID = "id"
		b.opts.MaxBufferSize = 4096
		b.closed = true

		b.Send(message.ConvertToComposer(level.Debug, "should fail"))
		assert.Empty(t, b.buffer)
		assert.Zero(t, b.bufferSize)
		assert.Nil(t, mc.logLines)
		assert.Equal(t, "cannot call Send on a closed Buildlogger Sender", ms.lastMessage)
	})
}

func TestClose(t *testing.T) {
	t.Run("CloseNonNilConn", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)

		assert.NoError(t, b.Close())
		b.closed = false
		b.conn = &grpc.ClientConn{}
		assert.Panics(t, func() { _ = b.Close() })
	})
	t.Run("EmptyBuffer", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.logID = "id"
		b.opts.SetExitCode(10)

		require.NoError(t, b.Close())
		assert.Equal(t, b.opts.logID, mc.logEndInfo.LogId)
		assert.Equal(t, b.opts.exitCode, mc.logEndInfo.ExitCode)
		assert.Equal(t, time.Now().Unix(), mc.logEndInfo.CompletedAt.Seconds)
		assert.True(t, b.closed)
	})
	t.Run("NonEmptyBuffer", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.logID = "id"
		b.opts.SetExitCode(2)
		logLine := &internal.LogLine{Timestamp: &timestamp.Timestamp{}, Data: "some data"}
		b.buffer = append(b.buffer, logLine)

		require.NoError(t, b.Close())
		assert.NotNil(t, mc.logEndInfo)
		assert.Equal(t, time.Now().Unix(), mc.logEndInfo.CompletedAt.Seconds)
		assert.Equal(t, b.opts.logID, mc.logEndInfo.LogId)
		assert.Equal(t, b.opts.exitCode, mc.logEndInfo.ExitCode)
		assert.NotNil(t, mc.logLines)
		assert.Equal(t, b.opts.logID, mc.logLines.LogId)
		assert.Equal(t, logLine, mc.logLines.Lines[0])
		assert.True(t, b.closed)
	})
	t.Run("NoopWhenClosed", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.closed = true

		require.NoError(t, b.Close())
		assert.Nil(t, mc.logEndInfo)
		assert.True(t, b.closed)
	})
	t.Run("RPCErrors", func(t *testing.T) {
		mc := &mockClient{appendErr: true}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		logLine := &internal.LogLine{Timestamp: &timestamp.Timestamp{}, Data: "some data"}
		b.buffer = append(b.buffer, logLine)

		assert.Error(t, b.Close())
		assert.Equal(t, "append error", ms.lastMessage)

		b.closed = false
		mc.appendErr = false
		mc.closeErr = true
		assert.Error(t, b.Close())
		assert.Equal(t, "close error", ms.lastMessage)
		assert.True(t, b.closed)
	})
}

func createSender(mc internal.BuildloggerClient, ms send.Sender) *buildlogger {
	return &buildlogger{
		ctx: context.TODO(),
		opts: &LoggerOptions{
			Project:     "project",
			Version:     "version",
			Variant:     "variant",
			TaskName:    "task_name",
			TaskID:      "task_id",
			Execution:   1,
			TestName:    "test_name",
			Trial:       2,
			ProcessName: "proc_name",
			Arguments:   map[string]string{"tag1": "val", "tag2": "val2"},
			Mainline:    true,
			Local:       ms,
			format:      internal.LogFormat_LOG_FORMAT_TEXT,
		},
		client: mc,
		buffer: []*internal.LogLine{},
		Base:   send.NewBase("test"),
	}
}

func newRandString(size int) string {
	b := make([]byte, size)
	_, _ = rand.Read(b)
	return string(b)
}

func startRPCService(ctx context.Context, service internal.BuildloggerServer, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	internal.RegisterBuildloggerServer(s, service)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}
