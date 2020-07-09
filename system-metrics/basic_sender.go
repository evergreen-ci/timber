package systemmetrics

import (
	"context"
	"net/http"
	"sync"

	"github.com/evergreen-ci/timber"
	"github.com/evergreen-ci/timber/internal"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const (
	defaultMaxBufferSize int = 1e7
)

// DataFormat describes the format of the system metrics data.
type DataFormat int32

// Valid DataFormat values.
const (
	DataFormatText DataFormat = 0
	DataFormatFTDC DataFormat = 1
	DataFormatBSON DataFormat = 2
	DataFormatJSON DataFormat = 3
	DataFormatCSV  DataFormat = 4
)

func (f DataFormat) validate() error {
	switch f {
	case DataFormatText, DataFormatFTDC, DataFormatBSON, DataFormatJSON, DataFormatCSV:
		return nil
	default:
		return errors.New("invalid data format specified")
	}
}

// CompressionType describes how the system metrics data is compressed.
type CompressionType int32

// Valid CompressionType values.
const (
	CompressionTypeNone  CompressionType = 0
	CompressionTypeTARGZ CompressionType = 1
	CompressionTypeZIP   CompressionType = 2
	CompressionTypeGZ    CompressionType = 3
	CompressionTypeXZ    CompressionType = 4
)

func (f CompressionType) validate() error {
	switch f {
	case CompressionTypeNone, CompressionTypeTARGZ, CompressionTypeZIP, CompressionTypeGZ, CompressionTypeXZ:
		return nil
	default:
		return errors.New("invalid compression type specified")
	}
}

// SchemaType describes how the time series data is stored.
type SchemaType int32

// Valid SchemaType values.
const (
	SchemaTypeRawEvents             SchemaType = 0
	SchemaTypeCollapsedEvents       SchemaType = 1
	SchemaTypeIntervalSummarization SchemaType = 2
	SchemaTypeHistogram             SchemaType = 3
)

func (f SchemaType) validate() error {
	switch f {
	case SchemaTypeRawEvents, SchemaTypeCollapsedEvents, SchemaTypeIntervalSummarization, SchemaTypeHistogram:
		return nil
	default:
		return errors.New("invalid schema type specified")
	}
}

type metricslogger struct {
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	opts       *SystemMetricsOptions
	conn       *grpc.ClientConn
	client     internal.CedarSystemMetricsClient
	buffer     []byte
	bufferSize int
	closed     bool
	*send.Base
}

// SystemMetricsOptions support the use and creation of a system metrics object.
type SystemMetricsOptions struct {
	// Unique information to identify the system metrics object.
	Project   string `bson:"project" json:"project" yaml:"project"`
	Version   string `bson:"version" json:"version" yaml:"version"`
	Variant   string `bson:"variant" json:"variant" yaml:"variant"`
	TaskName  string `bson:"task_name" json:"task_name" yaml:"task_name"`
	TaskID    string `bson:"task_id" json:"task_id" yaml:"task_id"`
	Execution int32  `bson:"execution" json:"execution" yaml:"execution"`
	Mainline  bool   `bson:"mainline" json:"mainline" yaml:"mainline"`

	// Data storage information for this object
	Format      DataFormat      `bson:"format" json:"format" yaml:"format"`
	Compression CompressionType `bson:"compression" json:"compression" yaml:"compression"`
	Schema      SchemaType      `bson:"schema" json:"schema" yaml:"schema"`

	// The number max number of bytes to buffer before sending log data
	// over rpc to cedar. Defaults to 10MB.
	MaxBufferSize int `bson:"max_buffer_size" json:"max_buffer_size" yaml:"max_buffer_size"`

	// The gRPC client connection. If nil, a new connection will be
	// established with the gRPC connection configuration.
	ClientConn *grpc.ClientConn `bson:"-" json:"-" yaml:"-"`

	// Configuration for gRPC client connection.
	APIKey   string      `bson:"api_key" json:"api_key" yaml:"api_key"`
	Username string      `bson:"username" json:"username" yaml:"username"`
	Client   http.Client `bson:"client" json:"client" yaml:"client"`

	systemMetricsID string
}

func (opts *SystemMetricsOptions) validate() error {
	if err := opts.Format.validate(); err != nil {
		return err
	}
	if err := opts.Compression.validate(); err != nil {
		return err
	}
	if err := opts.Schema.validate(); err != nil {
		return err
	}

	if opts.ClientConn == nil {
		if opts.APIKey == "" {
			return errors.New("must specify an API key when a client connection is not provided")
		}
		if opts.Username == "" {
			return errors.New("must specify a username when a client connection is not provided")
		}
	}

	if opts.MaxBufferSize == 0 {
		opts.MaxBufferSize = defaultMaxBufferSize
	}

	return nil
}

// GetSystemMetricsID returns the unique metricslogger log ID set after NewMetricsLogger is
// called.
func (opts *SystemMetricsOptions) GetSystemMetricsID() string { return opts.systemMetricsID }

// MakeLoggerWithContext returns system metrics logger backed by cedar using
// the passed in context.
func CreateSystemMetrics(name string, opts *SystemMetricsOptions) (*metricslogger, error) {
	ctx := context.Background()
	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "invalid cedar metricslogger options")
	}

	var conn *grpc.ClientConn
	var err error
	if opts.ClientConn == nil {
		rpcOpts := timber.DialCedarOptions{
			Username: opts.Username,
			APIKey:   opts.APIKey,
		}

		conn, err = timber.DialCedar(ctx, &opts.Client, rpcOpts)
		if err != nil {
			return nil, errors.Wrap(err, "problem dialing rpc server")
		}
		opts.ClientConn = conn
	}

	m := &metricslogger{
		ctx:    ctx,
		opts:   opts,
		conn:   conn,
		client: internal.NewCedarSystemMetricsClient(opts.ClientConn),
		buffer: []byte{},
		Base:   send.NewBase(name),
	}

	if err := m.createNewSystemMetrics(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	m.ctx = ctx
	m.cancel = cancel

	return m, nil
}

// Send sends the given byte slice to the cedar backend. This function buffers the data
// until the maximum allowed buffer size is reached, at which point the data
// in the buffer is sent to the cedar server via RPC. Send is thread safe.
func (m *metricslogger) AddSystemMetricsData(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("cannot call Send on a closed system metrics logger")
	}

	if m.bufferSize+len(data) > m.opts.MaxBufferSize {
		for total := m.bufferSize + len(data); total > m.opts.MaxBufferSize; total -= m.opts.MaxBufferSize {
			capacity := m.opts.MaxBufferSize - m.bufferSize
			m.buffer = append(m.buffer, data[:capacity]...)
			m.bufferSize += capacity
			data = data[capacity:]

			if err := m.flush(m.ctx); err != nil {
				return errors.Wrap(err, "problem flushing buffer")
			}
		}
	} else {
		m.buffer = append(m.buffer, data...)
		m.bufferSize += len(data)
	}

	m.buffer = append(m.buffer, data...)
	m.bufferSize += len(data)
	if m.bufferSize > m.opts.MaxBufferSize {
		if err := m.flush(m.ctx); err != nil {
			return errors.Wrap(err, "problem flushing buffer")
		}
	}

	return nil
}

// Flush flushes anything messages that may be in the buffer to cedar
// Buildlogger backend via RPC.
func (m *metricslogger) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	return m.flush(ctx)
}

// Close flushes anything that may be left in the underlying buffer and closes
// out the system metrics object with a completed at timestamp. If the gRPC
// client connection was created in NewMetricsLogger or MakeMetricsLogger,
// this connection is also closed. Close is thread safe but should only be called
// once no morecalls to Send are needed; after Close has been called any subsequent
// calls to Send will error. After the first call to Close subsequent calls will
// no-op.
func (m *metricslogger) CloseSystemMetrics() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.cancel()

	if m.closed {
		return nil
	}
	catcher := grip.NewBasicCatcher()

	if len(m.buffer) > 0 {
		if err := m.flush(m.ctx); err != nil {
			catcher.Add(errors.Wrap(err, "problem flushing buffer"))
		}
	}

	if !catcher.HasErrors() {
		endInfo := &internal.SystemMetricsSeriesEnd{
			Id: m.opts.systemMetricsID,
		}
		_, err := m.client.CloseMetrics(m.ctx, endInfo)
		catcher.Add(errors.Wrap(err, "problem closing system metrics object"))
	}

	if m.conn != nil {
		catcher.Add(m.conn.Close())
	}

	m.closed = true

	return catcher.Resolve()
}

func (m *metricslogger) createNewSystemMetrics() error {
	data := &internal.SystemMetrics{
		Info: &internal.SystemMetricsInfo{
			Project:   m.opts.Project,
			Version:   m.opts.Version,
			Variant:   m.opts.Variant,
			TaskName:  m.opts.TaskName,
			TaskId:    m.opts.TaskID,
			Execution: m.opts.Execution,
			Mainline:  m.opts.Mainline,
		},
		Artifact: &internal.SystemMetricsArtifactInfo{
			Format:      internal.DataFormat(m.opts.Format),
			Compression: internal.CompressionType(m.opts.Compression),
			Schema:      internal.SchemaType(m.opts.Schema),
		},
	}
	resp, err := m.client.CreateSystemMetricRecord(m.ctx, data)
	if err != nil {
		return errors.Wrap(err, "problem creating system metrics object")
	}
	m.opts.systemMetricsID = resp.Id

	return nil
}

func (m *metricslogger) flush(ctx context.Context) error {
	_, err := m.client.AddSystemMetrics(ctx, &internal.SystemMetricsData{
		Id:   m.opts.systemMetricsID,
		Data: m.buffer,
	})
	if err != nil {
		return err
	}

	m.buffer = []byte{}
	m.bufferSize = 0

	return nil
}
