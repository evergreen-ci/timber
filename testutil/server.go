package testutil

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/evergreen-ci/juniper/gopb"
	"github.com/evergreen-ci/timber"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// MockCedarServer sets up a mock Cedar server for sending metrics, test
// results, and logs using gRPC.
type MockCedarServer struct {
	TestResults *MockTestResultsServer
	Buildlogger *MockBuildloggerServer
	Health      *MockHealthServer
	DialOpts    timber.DialCedarOptions
}

// NewMockCedarServer will return a new MockCedarServer listening on a port
// near the provided port.
func NewMockCedarServer(ctx context.Context, basePort int) (*MockCedarServer, error) {
	srv := &MockCedarServer{
		TestResults: &MockTestResultsServer{},
		Buildlogger: &MockBuildloggerServer{},
		Health:      &MockHealthServer{},
	}
	port := GetPortNumber(basePort)

	srv.DialOpts = timber.DialCedarOptions{
		BaseAddress: "localhost",
		RPCPort:     strconv.Itoa(port),
	}

	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterCedarTestResultsServer(s, srv.TestResults)
	gopb.RegisterBuildloggerServer(s, srv.Buildlogger)
	gopb.RegisterHealthServer(s, srv.Health)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// NewMockCedarServerWithDialOpts will return a new MockCedarServer listening
// on the port and URL from the specified dial options.
func NewMockCedarServerWithDialOpts(ctx context.Context, opts timber.DialCedarOptions) (*MockCedarServer, error) {
	srv := &MockCedarServer{
		TestResults: &MockTestResultsServer{},
		Buildlogger: &MockBuildloggerServer{},
		Health:      &MockHealthServer{},
	}
	srv.DialOpts = opts
	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterCedarTestResultsServer(s, srv.TestResults)
	gopb.RegisterBuildloggerServer(s, srv.Buildlogger)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// Address returns the address the server is listening on.
func (ms *MockCedarServer) Address() string {
	return fmt.Sprintf("%s:%s", ms.DialOpts.BaseAddress, ms.DialOpts.RPCPort)
}

// MockTestResultsServer sets up a mock Cedar server for sending test results
// using gRPC.
type MockTestResultsServer struct {
	CreateErr     bool
	AddErr        bool
	StreamErr     bool
	CloseErr      bool
	Create        *gopb.TestResultsInfo
	Results       map[string][]*gopb.TestResults
	StreamResults map[string][]*gopb.TestResults
	Close         *gopb.TestResultsEndInfo
	DialOpts      timber.DialCedarOptions

	// UnimplementedCedarTestResultsServer must be embedded for forward
	// compatibility. See gopb.test_results_grpc.pb.go for more
	// information.
	gopb.UnimplementedCedarTestResultsServer
}

// Address returns the address the server is listening on.
func (ms *MockTestResultsServer) Address() string {
	return fmt.Sprintf("%s:%s", ms.DialOpts.BaseAddress, ms.DialOpts.RPCPort)
}

// NewMockTestResultsServer returns a new MockTestResultsServer listening on a
// port near the provided port.
func NewMockTestResultsServer(ctx context.Context, basePort int) (*MockTestResultsServer, error) {
	srv := &MockTestResultsServer{}
	port := GetPortNumber(basePort)

	srv.DialOpts = timber.DialCedarOptions{
		BaseAddress: "localhost",
		RPCPort:     strconv.Itoa(port),
	}

	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterCedarTestResultsServer(s, srv)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// NewMockTestResultsServerWithDialOpts returns a new MockTestResultsServer
// listening on the port and URL from the specified dial options.
func NewMockTestResultsServerWithDialOpts(ctx context.Context, opts timber.DialCedarOptions) (*MockTestResultsServer, error) {
	srv := &MockTestResultsServer{}
	srv.DialOpts = opts
	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterCedarTestResultsServer(s, srv)

	go func() {
		grip.Error(errors.Wrap(s.Serve(lis), "running server"))
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// CreateTestResultsRecord returns an error if CreateErr is true, otherwise it
// sets Create to the input.
func (m *MockTestResultsServer) CreateTestResultsRecord(_ context.Context, in *gopb.TestResultsInfo) (*gopb.TestResultsResponse, error) {
	if m.CreateErr {
		return nil, errors.New("create error")
	}
	m.Create = in
	return &gopb.TestResultsResponse{TestResultsRecordId: utility.RandomString()}, nil
}

// AddTestResults returns an error if AddErr is true, otherwise it adds the
// input to Results.
func (m *MockTestResultsServer) AddTestResults(_ context.Context, in *gopb.TestResults) (*gopb.TestResultsResponse, error) {
	if m.AddErr {
		return nil, errors.New("add error")
	}
	if m.Results == nil {
		m.Results = make(map[string][]*gopb.TestResults)
	}
	m.Results[in.TestResultsRecordId] = append(m.Results[in.TestResultsRecordId], in)
	return &gopb.TestResultsResponse{TestResultsRecordId: in.TestResultsRecordId}, nil
}

// StreamTestResults returns a not implemented error.
func (ms *MockTestResultsServer) StreamTestResults(gopb.CedarTestResults_StreamTestResultsServer) error {
	return errors.New("not implemented")
}

// CloseTestResults returns an error if CloseErr is true, otherwise it sets
// Close to the input.
func (m *MockTestResultsServer) CloseTestResultsRecord(_ context.Context, in *gopb.TestResultsEndInfo) (*gopb.TestResultsResponse, error) {
	if m.CloseErr {
		return nil, errors.New("close error")
	}
	m.Close = in
	return &gopb.TestResultsResponse{TestResultsRecordId: in.TestResultsRecordId}, nil
}

// MockBuildloggerServer sets up a mock Cedar server for testing buildlogger
// logs using gRPC.
type MockBuildloggerServer struct {
	Mu        sync.Mutex
	CreateErr bool
	AppendErr bool
	CloseErr  bool
	Create    *gopb.LogData
	Data      map[string][]*gopb.LogLines
	Close     *gopb.LogEndInfo
	DialOpts  timber.DialCedarOptions

	// UnimplementedBuildloggerServer must be embedded for forward
	// compatibility. See gopb.buildlogger_grpc.pb.go for more information.
	gopb.UnimplementedBuildloggerServer
}

// NewMockBuildloggerServer returns a new MockBuildloggerServer listening on a
// port near the provided port.
func NewMockBuildloggerServer(ctx context.Context, basePort int) (*MockBuildloggerServer, error) {
	srv := &MockBuildloggerServer{
		Data: make(map[string][]*gopb.LogLines),
	}
	port := GetPortNumber(basePort)

	srv.DialOpts = timber.DialCedarOptions{
		BaseAddress: "localhost",
		RPCPort:     strconv.Itoa(port),
	}

	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterBuildloggerServer(s, srv)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// NewMockBuildloggerServerWithDialOpts returns a new MockBuildloggerServer
// listening on the port and URL from the specified dial options.
func NewMockBuildloggerServerWithDialOpts(ctx context.Context, opts timber.DialCedarOptions) (*MockBuildloggerServer, error) {
	srv := &MockBuildloggerServer{}
	srv.DialOpts = opts
	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterBuildloggerServer(s, srv)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// Address returns the address the server is listening on.
func (ms *MockBuildloggerServer) Address() string {
	return fmt.Sprintf("%s:%s", ms.DialOpts.BaseAddress, ms.DialOpts.RPCPort)
}

// CreateLog returns an error if CreateErr is true, otherwise it sets Create to
// the input.
func (ms *MockBuildloggerServer) CreateLog(_ context.Context, in *gopb.LogData) (*gopb.BuildloggerResponse, error) {
	ms.Mu.Lock()
	defer ms.Mu.Unlock()

	if ms.CreateErr {
		return nil, errors.New("create error")
	}

	ms.Create = in
	return &gopb.BuildloggerResponse{}, nil
}

// AppendLogLines returns an error if AppendErr is true, otherwise it adds the
// input to Data.
func (ms *MockBuildloggerServer) AppendLogLines(_ context.Context, in *gopb.LogLines) (*gopb.BuildloggerResponse, error) {
	ms.Mu.Lock()
	defer ms.Mu.Unlock()

	if ms.AppendErr {
		return nil, errors.New("append error")
	}

	if ms.Data == nil {
		ms.Data = make(map[string][]*gopb.LogLines)
	}
	ms.Data[in.LogId] = append(ms.Data[in.LogId], in)

	return &gopb.BuildloggerResponse{LogId: in.LogId}, nil
}

// StreamLogLines returns a not implemented error.
func (ms *MockBuildloggerServer) StreamLogLines(in gopb.Buildlogger_StreamLogLinesServer) error {
	return errors.New("not implemented")
}

// CloseLog returns an error if CloseErr is true, otherwise it sets Close to
// the input.
func (ms *MockBuildloggerServer) CloseLog(_ context.Context, in *gopb.LogEndInfo) (*gopb.BuildloggerResponse, error) {
	ms.Mu.Lock()
	defer ms.Mu.Unlock()

	if ms.CloseErr {
		return nil, errors.New("close error")
	}

	ms.Close = in
	return &gopb.BuildloggerResponse{LogId: in.LogId}, nil
}

// MockHealthServer sets up a mock Cedar server for testing the health check
// gRPC route.
type MockHealthServer struct {
	Mu       sync.Mutex
	Status   *gopb.HealthCheckResponse_ServingStatus
	Err      bool
	DialOpts timber.DialCedarOptions

	// UnimplementedHealthServer must be embedded for forward
	// compatibility. See gopb.health_grpc.pb.go for more information.
	gopb.UnimplementedHealthServer
}

// NewMockHealthServer returns a new MockHealthServer listening on a port near
// near the provided port.
func NewMockHealthServer(ctx context.Context, basePort int) (*MockHealthServer, error) {
	srv := &MockHealthServer{}
	port := GetPortNumber(basePort)
	srv.DialOpts = timber.DialCedarOptions{
		BaseAddress: "localhost",
		RPCPort:     strconv.Itoa(port),
	}

	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterHealthServer(s, srv)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// NewMockHealthServerWithDialOpts returns a new MockHealthServer listening on
// the port and URL from the specified dial options.
func NewMockHealthServerWithDialOpts(ctx context.Context, opts timber.DialCedarOptions) (*MockHealthServer, error) {
	srv := &MockHealthServer{}
	srv.DialOpts = opts
	lis, err := net.Listen("tcp", srv.Address())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := grpc.NewServer()
	gopb.RegisterHealthServer(s, srv)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return srv, nil
}

// Address returns the address the server is listening on.
func (ms *MockHealthServer) Address() string {
	return fmt.Sprintf("%s:%s", ms.DialOpts.BaseAddress, ms.DialOpts.RPCPort)
}

// Check returns (in the following order of precedence) an error if Err is
// true, Status if it is not empty, and "SERVING" otherwise.
func (ms *MockHealthServer) Check(_ context.Context, in *gopb.HealthCheckRequest) (*gopb.HealthCheckResponse, error) {
	ms.Mu.Lock()
	defer ms.Mu.Unlock()

	if ms.Err {
		return nil, errors.New("health check error")
	}

	status := gopb.HealthCheckResponse_SERVING
	if ms.Status != nil {
		status = *ms.Status
	}

	return &gopb.HealthCheckResponse{Status: status}, nil
}
