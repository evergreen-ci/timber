package testresults

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/timber"
	"github.com/evergreen-ci/timber/internal"
	"github.com/golang/protobuf/ptypes"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// Client provides a wrapper around a gRPC client for sending test results to Cedar.
type Client struct {
	client    internal.CedarTestResultsClient
	closeConn func() error
	closed    bool
}

// NewClient returns a Client to send test results to Cedar. If authentication credentials are not
// specified, then an insecure connection will be established with the specified address and port.
func NewClient(ctx context.Context, opts timber.ConnectionOptions) (*Client, error) {
	var conn *grpc.ClientConn
	var err error

	if err = opts.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid connection options")
	}

	if opts.DialOpts.APIKey == "" {
		addr := fmt.Sprintf("%s:%s", opts.DialOpts.BaseAddress, opts.DialOpts.RPCPort)
		conn, err = grpc.DialContext(ctx, addr, grpc.WithInsecure())
	} else {
		conn, err = timber.DialCedar(ctx, &opts.Client, opts.DialOpts)
	}
	if err != nil {
		return nil, errors.Wrap(err, "problem dialing rpc server")
	}

	s := &Client{
		client:    internal.NewCedarTestResultsClient(conn),
		closeConn: conn.Close,
	}
	return s, nil
}

// NewCNewClientWithExistingConnection returns a Client to send test results to Cedar using the
// given client connection. The given client connection's lifetime will not be managed by this
// client.
func NewClientWithExistingConnection(ctx context.Context, conn *grpc.ClientConn) (*Client, error) {
	if conn == nil {
		return nil, errors.New("must provide an existing client connection")
	}

	s := &Client{
		client:    internal.NewCedarTestResultsClient(conn),
		closeConn: func() error { return nil },
	}
	return s, nil
}

type CreateOptions struct {
	Project     string `bson:"project" json:"project" yaml:"project"`
	Version     string `bson:"version" json:"version" yaml:"version"`
	Variant     string `bson:"variant" json:"variant" yaml:"variant"`
	TaskID      string `bson:"task_id" json:"task_id" yaml:"task_id"`
	TaskName    string `bson:"task_name" json:"task_name" yaml:"task_name"`
	Execution   int32  `bson:"execution" json:"execution" yaml:"execution"`
	RequestType string `bson:"request_type" json:"request_type" yaml:"request_type"`
	Mainline    bool   `bson:"mainline" json:"mainline" yaml:"mainline"`
}

func (opts CreateOptions) Export() *internal.TestResultsInfo {
	return &internal.TestResultsInfo{
		Project:     opts.Project,
		Version:     opts.Version,
		Variant:     opts.Variant,
		TaskName:    opts.TaskName,
		TaskId:      opts.TaskID,
		Execution:   opts.Execution,
		RequestType: opts.RequestType,
		Mainline:    opts.Mainline,
	}
}

// CreateRecord creates a new metadata record in Cedar with the given options.
func (c *Client) CreateRecord(ctx context.Context, opts CreateOptions) (string, error) {
	resp, err := c.client.CreateTestResultsRecord(ctx, opts.Export())
	if err != nil {
		return "", errors.WithStack(err)
	}
	return resp.TestResultsRecordId, nil
}

// AddResults adds a set of test results to the record.
func (c *Client) AddResults(ctx context.Context, r Results) error {
	if err := r.validate(); err != nil {
		return errors.Wrap(err, "invalid test results")
	}

	exported, err := r.Export()
	if err != nil {
		return errors.Wrap(err, "converting test results to protobuf type")
	}

	if _, err := c.client.AddTestResults(ctx, exported); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// CloseRecord marks a record as completed.
func (c *Client) CloseRecord(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	if _, err := c.client.CloseTestResultsRecord(ctx, &internal.TestResultsEndInfo{TestResultsRecordId: id}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// CloseClient closes the client connection if it was created via NewClient. If an existing
// connection was used to create the client, it will not be closed.
func (c *Client) CloseClient() error {
	if c.closed {
		return nil
	}
	if c.closeConn == nil {
		return nil
	}
	return c.closeConn()
}

// Results represent a set of test results.
type Results struct {
	ID      string
	Results []Result
}

func (r Results) validate() error {
	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(r.ID == "", "must specify test result ID")
	return catcher.Resolve()
}

// Export converts Results into the equivalent protobuf TestResults.
func (r Results) Export() (*internal.TestResults, error) {
	var results []*internal.TestResult
	for _, res := range r.Results {
		exported, err := res.Export()
		if err != nil {
			return nil, errors.Wrap(err, "converting test result")
		}
		results = append(results, exported)
	}
	return &internal.TestResults{
		TestResultsRecordId: r.ID,
		Results:             results,
	}, nil
}

// Result represents a single test result.
type Result struct {
	Name        string
	Trial       int32
	Status      string
	LogURL      string
	LineNum     int32
	TaskCreated time.Time
	TestStarted time.Time
	TestEnded   time.Time
}

// Export converts a Result into the equivalent protobuf TestResult.
func (r Result) Export() (*internal.TestResult, error) {
	created, err := ptypes.TimestampProto(r.TaskCreated)
	if err != nil {
		return nil, errors.Wrap(err, "converting create timestamp")
	}
	started, err := ptypes.TimestampProto(r.TestStarted)
	if err != nil {
		return nil, errors.Wrap(err, "converting start timestamp")
	}
	ended, err := ptypes.TimestampProto(r.TestEnded)
	if err != nil {
		return nil, errors.Wrap(err, "converting end timestamp")
	}
	return &internal.TestResult{
		TestName:       r.Name,
		Trial:          r.Trial,
		Status:         r.Status,
		LogUrl:         r.LogURL,
		LineNum:        r.LineNum,
		TaskCreateTime: created,
		TestStartTime:  started,
		TestEndTime:    ended,
	}, nil
}
