package systemmetrics

import (
	"context"
	"testing"

	"github.com/evergreen-ci/timber/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type mockClient struct {
	info  *internal.SystemMetrics
	data  *internal.SystemMetricsData
	close *internal.SystemMetricsSeriesEnd
}

func (mc mockClient) CreateSystemMetricRecord(ctx context.Context, in *internal.SystemMetrics, opts ...grpc.CallOption) (*internal.SystemMetricsResponse, error) {
	mc.info = in
	return &internal.SystemMetricsResponse{
		Id: "ID",
	}, nil
}

func (mc mockClient) AddSystemMetrics(ctx context.Context, in *internal.SystemMetricsData, opts ...grpc.CallOption) (*internal.SystemMetricsResponse, error) {
	mc.data = in
	return &internal.SystemMetricsResponse{
		Id: "ID",
	}, nil
}

func (mc mockClient) StreamSystemMetrics(ctx context.Context, opts ...grpc.CallOption) (internal.CedarSystemMetrics_StreamSystemMetricsClient, error) {
	return nil, nil
}

func (mc mockClient) CloseMetrics(ctx context.Context, in *internal.SystemMetricsSeriesEnd, opts ...grpc.CallOption) (*internal.SystemMetricsResponse, error) {
	mc.close = in
	return &internal.SystemMetricsResponse{
		Id: "ID",
	}, nil
}

func TestNewSystemMetricsClient(t *testing.T) {
	t.Run("ExistingConnection", func(t *testing.T) {
	})
	t.Run("ValidOptions", func(t *testing.T) {
	})
	t.Run("InvalidOptions", func(t *testing.T) {
	})
}

func TestCreateSystemMetricsRecord(t *testing.T) {
	t.Run("ValidOptions", func(t *testing.T) {
	})
	t.Run("InvalidOptions", func(t *testing.T) {
	})
}

func TestAddSystemMetrics(t *testing.T) {
	ctx := context.Background()
	t.Run("ValidOptions", func(t *testing.T) {
		mc := mockClient{}
		s := SystemMetricsClient{
			client: mc,
		}
		require.NoError(t, s.AddSystemMetrics(ctx, "ID", []byte("Test byte string")))
		assert.Equal(t, mc.data, internal.SystemMetricsData{
			Id:   "ID",
			Data: []byte("Test byte string"),
		})
	})
	t.Run("InvalidOptions", func(t *testing.T) {
	})
}

func TestCloseSystemMetrics(t *testing.T) {
	t.Run("ValidOptions", func(t *testing.T) {
	})
	t.Run("InvalidOptions", func(t *testing.T) {
	})
}
