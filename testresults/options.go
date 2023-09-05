package testresults

import (
	"time"

	"github.com/evergreen-ci/juniper/gopb"
	"github.com/mongodb/grip"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateOptions represent options to create a new test results record.
type CreateOptions struct {
	Project         string `bson:"project" json:"project" yaml:"project"`
	Version         string `bson:"version" json:"version" yaml:"version"`
	Variant         string `bson:"variant" json:"variant" yaml:"variant"`
	TaskID          string `bson:"task_id" json:"task_id" yaml:"task_id"`
	TaskName        string `bson:"task_name" json:"task_name" yaml:"task_name"`
	DisplayTaskName string `bson:"display_task_name" json:"display_task_name" yaml:"display_task_name"`
	DisplayTaskID   string `bson:"display_task_id" json:"display_task_id" yaml:"display_task_id"`
	Execution       int32  `bson:"execution" json:"execution" yaml:"execution"`
	RequestType     string `bson:"request_type" json:"request_type" yaml:"request_type"`
	Mainline        bool   `bson:"mainline" json:"mainline" yaml:"mainline"`
}

func (opts CreateOptions) export() *gopb.TestResultsInfo {
	return &gopb.TestResultsInfo{
		Project:         opts.Project,
		Version:         opts.Version,
		Variant:         opts.Variant,
		TaskName:        opts.TaskName,
		TaskId:          opts.TaskID,
		DisplayTaskName: opts.DisplayTaskName,
		DisplayTaskId:   opts.DisplayTaskID,
		Execution:       opts.Execution,
		RequestType:     opts.RequestType,
		Mainline:        opts.Mainline,
	}
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

// export converts Results into the equivalent protobuf TestResults.
func (r Results) export() *gopb.TestResults {
	var results []*gopb.TestResult
	for _, res := range r.Results {
		results = append(results, res.export())
	}
	return &gopb.TestResults{
		TestResultsRecordId: r.ID,
		Results:             results,
	}
}

// Result represents a single test result.
type Result struct {
	TestName        string    `bson:"test_name" json:"test_name" yaml:"test_name"`
	DisplayTestName string    `bson:"display_test_name" json:"display_test_name" yaml:"display_test_name"`
	GroupID         string    `bson:"group_id" json:"group_id" yaml:"group_id"`
	Trial           int32     `bson:"trial" json:"trial" yaml:"trial"`
	Status          string    `bson:"status" json:"status" yaml:"status"`
	LogInfo         *LogInfo  `bson:"log_info" json:"log_info" yaml:"log_info"`
	TaskCreated     time.Time `bson:"task_created" json:"task_created" yaml:"task_created"`
	TestStarted     time.Time `bson:"test_started" json:"test_started" yaml:"test_started"`
	TestEnded       time.Time `bson:"test_ended" json:"test_ended" yaml:"test_ended"`

	// Legacy test log fields.
	LogTestName string `bson:"log_test_name" json:"log_test_name" yaml:"log_test_name"`
	LogURL      string `bson:"log_url" json:"log_url" yaml:"log_url"`
	RawLogURL   string `bson:"raw_log_url" json:"raw_log_url" yaml:"raw_log_url"`
	LineNum     int32  `bson:"line_num" json:"line_num" yaml:"line_num"`
}

// export converts a Result into the equivalent protobuf TestResult.
func (r Result) export() *gopb.TestResult {
	return &gopb.TestResult{
		TestName:        r.TestName,
		DisplayTestName: r.DisplayTestName,
		GroupId:         r.GroupID,
		Trial:           r.Trial,
		Status:          r.Status,
		LogInfo:         r.LogInfo.export(),
		LogTestName:     r.LogTestName,
		LogUrl:          r.LogURL,
		RawLogUrl:       r.RawLogURL,
		LineNum:         r.LineNum,
		TaskCreateTime:  timestamppb.New(r.TaskCreated),
		TestStartTime:   timestamppb.New(r.TestStarted),
		TestEndTime:     timestamppb.New(r.TestEnded),
	}
}

// LogInfo describes a metadata for a result's log stored using Evergreen
// logging.
type LogInfo struct {
	LogName       string
	LogsToMerge   []string
	LineNum       int32
	RenderingType *string
	Version       int32
}

// export converts LogInfo into the equivalent protobuf TestLogInfo.
func (li *LogInfo) export() *gopb.TestLogInfo {
	if li == nil {
		return nil
	}

	return &gopb.TestLogInfo{
		LogName:       li.LogName,
		LogsToMerge:   li.LogsToMerge,
		LineNum:       li.LineNum,
		RenderingType: li.RenderingType,
		Version:       li.Version,
	}
}
