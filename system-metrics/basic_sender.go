package systemmetrics

import (
	"context"
	"net/http"

	"github.com/evergreen-ci/timber"
	"github.com/evergreen-ci/timber/internal"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

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

type systemMetricsClient struct {
	cancel context.CancelFunc
	client internal.CedarSystemMetricsClient
}

type ConnectionOptions struct {
	APIKey   string
	Username string
	Client   http.Client
}

func NewSystemMetricsClient(ctx context.Context, clientConn *grpc.ClientConn, opts ConnectionOptions) (*systemMetricsClient, error) {
	var conn *grpc.ClientConn
	var err error

	ctx, cancel := context.WithCancel(ctx)
	if clientConn == nil {
		if opts.APIKey == "" {
			return nil, errors.New("must specify an API key when a client connection is not provided")
		}
		if opts.Username == "" {
			return nil, errors.New("must specify a username when a client connection is not provided")
		}
		rpcOpts := timber.DialCedarOptions{
			Username: opts.Username,
			APIKey:   opts.APIKey,
		}

		conn, err = timber.DialCedar(ctx, &opts.Client, rpcOpts)
		if err != nil {
			return nil, errors.Wrap(err, "problem dialing rpc server")
		}
	}

	s := &systemMetricsClient{
		client: internal.NewCedarSystemMetricsClient(conn),
		cancel: cancel,
	}
	return s, nil
}

// SystemMetricsOptions support the use and creation of a system metrics object.
type SystemMetricsOptions struct {
	// Unique information to identify the system metrics object.
	Project   string `bson:"project" json:"project" yaml:"project"`
	Version   string `bson:"version" json:"version" yaml:"version"`
	Variant   string `bson:"variant" json:"variant" yaml:"variant"`
	TaskName  string `bson:"task_name" json:"task_name" yaml:"task_name"`
	TaskId    string `bson:"task_id" json:"task_id" yaml:"task_id"`
	Execution int32  `bson:"execution" json:"execution" yaml:"execution"`
	Mainline  bool   `bson:"mainline" json:"mainline" yaml:"mainline"`

	// Data storage information for this object
	Compression CompressionType `bson:"compression" json:"compression" yaml:"compression"`
	Schema      SchemaType      `bson:"schema" json:"schema" yaml:"schema"`
}

// CreateSystemMetrics creates a system metrics metadata object in cedar with
// the provided info, along with setting the created_at timestamp.
func (s *systemMetricsClient) CreateSystemMetricRecord(ctx context.Context, opts SystemMetricsOptions) (string, error) {
	// validation
	if err := opts.Compression.validate(); err != nil {
		return "", err
	}
	if err := opts.Schema.validate(); err != nil {
		return "", err
	}

	resp, err := s.client.CreateSystemMetricRecord(ctx, createSystemMetrics(opts))
	if err != nil {
		return "", errors.Wrap(err, "problem creating system metrics object")
	}

	return resp.Id, nil
}

// AddSystemMetricsData sends the given byte slice to the cedar backend for the
// system metrics object with the corresponding id.
func (s *systemMetricsClient) AddSystemMetrics(ctx context.Context, id string, data []byte) error {
	if id == "" {
		return errors.New("must specify id of system metrics object")
	}
	if len(data) == 0 {
		return nil
	}

	_, err := s.client.AddSystemMetrics(ctx, &internal.SystemMetricsData{
		Id:   id,
		Data: data,
	})
	return err
}

// StreamSystemMetrics is currently a no-op, will be implemented later.
func (s *systemMetricsClient) StreamSystemMetrics(ctx context.Context, id string, data []byte) error {
	return nil
}

// CloseMetrics will add the completed_at timestamp to the system metrics object
// in cedar with the corresponding id.
func (s *systemMetricsClient) CloseSystemMetrics(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("must specify id of system metrics object")
	}

	endInfo := &internal.SystemMetricsSeriesEnd{
		Id: id,
	}
	_, err := s.client.CloseMetrics(ctx, endInfo)
	return err
}

func createSystemMetrics(opts SystemMetricsOptions) *internal.SystemMetrics {
	return &internal.SystemMetrics{
		Info: &internal.SystemMetricsInfo{
			Project:   opts.Project,
			Version:   opts.Version,
			Variant:   opts.Variant,
			TaskName:  opts.TaskName,
			TaskId:    opts.TaskId,
			Execution: opts.Execution,
			Mainline:  opts.Mainline,
		},
		Artifact: &internal.SystemMetricsArtifactInfo{
			Compression: internal.CompressionType(opts.Compression),
			Schema:      internal.SchemaType(opts.Schema),
		},
	}
}
