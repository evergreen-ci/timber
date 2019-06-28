package buildlogger

import (
	"context"
	"crypto/rand"
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

type mockClient struct {
	logData    *internal.LogData
	logLines   *internal.LogLines
	logEndInfo *internal.LogEndInfo

	createErr bool
	appendErr bool
	closeErr  bool
}

func (mc *mockClient) CreateLog(ctx context.Context, in *internal.LogData, _ ...grpc.CallOption) (*internal.BuildloggerResponse, error) {
	if mc.createErr {
		return nil, errors.New("create log error")
	}

	mc.logData = in

	return &internal.BuildloggerResponse{LogId: in.Info.TestName}, nil
}

func (mc *mockClient) AppendLogLines(ctx context.Context, in *internal.LogLines, _ ...grpc.CallOption) (*internal.BuildloggerResponse, error) {
	if mc.appendErr {
		return nil, errors.New("append error")
	}

	mc.logLines = in

	return &internal.BuildloggerResponse{LogId: in.LogId}, nil
}

func (*mockClient) StreamLog(_ context.Context, _ ...grpc.CallOption) (internal.Buildlogger_StreamLogClient, error) {
	return nil, nil
}

func (mc *mockClient) CloseLog(ctx context.Context, in *internal.LogEndInfo, _ ...grpc.CallOption) (*internal.BuildloggerResponse, error) {
	if mc.closeErr {
		return nil, errors.New("close error")
	}

	mc.logEndInfo = in

	return &internal.BuildloggerResponse{LogId: in.LogId}, nil
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

func TestBuildloggerOptionsValidate(t *testing.T) {
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
		assert.Equal(t, "create log error", ms.lastMessage)
	})
}

func TestSend(t *testing.T) {
	t.Run("RespectsPriority", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)

		b.SetLevel(send.LevelInfo{Default: level.Debug, Threshold: level.Emergency})
		m := message.ConvertToComposer(level.Alert, "alert")
		b.Send(m)
		assert.Empty(t, b.buffer)
		m = message.ConvertToComposer(level.Emergency, "emergency")
		b.Send(m)
		require.NotEmpty(t, b.buffer)
		assert.Equal(t, m.String(), b.buffer[len(b.buffer)-1].Data)

		b.SetLevel(send.LevelInfo{Default: level.Debug, Threshold: level.Debug})
		m = message.ConvertToComposer(level.Debug, "debug")
		b.Send(m)
		require.NotEmpty(t, b.buffer)
		assert.Equal(t, m.String(), b.buffer[len(b.buffer)-1].Data)
	})
	t.Run("FlushAtCapacity", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.logID = "id"
		b.opts.MaxBufferSize = 4096
		size := 256
		messages := []message.Composer{}

		for b.bufferSize < b.opts.MaxBufferSize {
			m := message.ConvertToComposer(level.Debug, newRandString(size))
			messages = append(messages, m)

			b.Send(m)
			require.Empty(t, ms.lastMessage)
			assert.Nil(t, mc.logLines)
			assert.Equal(t, time.Now().Unix(), b.buffer[len(b.buffer)-1].Timestamp.Seconds)
			assert.Equal(t, m.String(), b.buffer[len(b.buffer)-1].Data)
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
			assert.Equal(t, messages[i].String(), mc.logLines.Lines[i].Data)
		}
	})
	t.Run("RPCError", func(t *testing.T) {
		mc := &mockClient{appendErr: true}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		b.opts.MaxBufferSize = 20

		m1 := message.ConvertToComposer(level.Debug, newRandString(b.opts.MaxBufferSize/2))
		m2 := message.ConvertToComposer(level.Debug, newRandString(b.opts.MaxBufferSize/2+1))
		b.Send(m1)
		b.Send(m2)
		assert.Len(t, b.buffer, 1)
		assert.Equal(t, b.opts.MaxBufferSize/2, b.bufferSize)
		assert.Equal(t, "append error", ms.lastMessage)
	})
}

func TestClose(t *testing.T) {
	t.Run("CloseNonNilConn", func(t *testing.T) {
		mc := &mockClient{}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)

		assert.NoError(t, b.Close())
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
	})
	t.Run("RPCErrors", func(t *testing.T) {
		mc := &mockClient{appendErr: true}
		ms := &mockSender{Base: send.NewBase("test")}
		b := createSender(mc, ms)
		logLine := &internal.LogLine{Timestamp: &timestamp.Timestamp{}, Data: "some data"}
		b.buffer = append(b.buffer, logLine)

		assert.Error(t, b.Close())
		assert.Equal(t, "append error", ms.lastMessage)

		mc.appendErr = false
		mc.closeErr = true
		assert.Error(t, b.Close())
		assert.Equal(t, "close error", ms.lastMessage)
	})
}

func createSender(mc internal.BuildloggerClient, ms send.Sender) *buildlogger {
	return &buildlogger{
		ctx: context.TODO(),
		opts: &BuildloggerOptions{
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
