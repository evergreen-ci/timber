package testutil

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/evergreen-ci/timber"
	"github.com/evergreen-ci/timber/internal"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type MockMetricsServer struct {
	CreateErr bool
	AddErr    bool
	CloseErr  bool
	Info      bool
	Data      bool
	Close     bool
	DialOpts  timber.DialCedarOptions
}

func (ms *MockMetricsServer) CreateSystemMetricsRecord(_ context.Context, in *internal.SystemMetrics) (*internal.SystemMetricsResponse, error) {
	if ms.CreateErr {
		return nil, errors.New("create error")
	}
	ms.Info = true
	return &internal.SystemMetricsResponse{
		Id: "ID",
	}, nil
}

func (ms *MockMetricsServer) AddSystemMetrics(_ context.Context, in *internal.SystemMetricsData) (*internal.SystemMetricsResponse, error) {
	if ms.AddErr {
		return nil, errors.New("add error")
	}
	ms.Data = true
	return &internal.SystemMetricsResponse{
		Id: "ID",
	}, nil
}

func (ms *MockMetricsServer) StreamSystemMetrics(internal.CedarSystemMetrics_StreamSystemMetricsServer) error {
	return nil
}

func (ms *MockMetricsServer) CloseMetrics(_ context.Context, in *internal.SystemMetricsSeriesEnd) (*internal.SystemMetricsResponse, error) {
	if ms.CloseErr {
		return nil, errors.New("close error")
	}
	ms.Close = true
	return &internal.SystemMetricsResponse{
		Id: "ID",
	}, nil
}

func (ms *MockMetricsServer) Address() string {
	return fmt.Sprintf("%s:%s", ms.DialOpts.BaseAddress, ms.DialOpts.RPCPort)
}

func NewMockMetricsServer(ctx context.Context, basePort int) (*MockMetricsServer, error) {
	srv := &MockMetricsServer{}
	port := GetPortNumber(basePort)
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	srv.DialOpts = timber.DialCedarOptions{
		BaseAddress: "localhost",
		RPCPort:     strconv.Itoa(port),
	}

	s := grpc.NewServer()
	internal.RegisterCedarSystemMetricsServer(s, srv)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}
