package testresults

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// GetSampleOptions specify the required and optional information to create the test
// results HTTP GET request to Cedar.
type GetSampleOptions struct {
	Cedar timber.GetOptions

	// Request information. See Cedar's REST documentation for more
	// information:
	// `https://github.com/evergreen-ci/cedar/wiki/Rest-V1-Usage`.
	SampleOptions TestSampleOptions
}

// TestSampleOptions specifies the tasks to get the sample for
// and regexes to filter the test names by.
type TestSampleOptions struct {
	Tasks        []TaskInfo `json:"tasks"`
	RegexFilters []string   `json:"regex_filters,omitempty"`
}

func (opts TestSampleOptions) validate() error {
	catcher := grip.NewBasicCatcher()

	if len(opts.Tasks) == 0 {
		catcher.Add(errors.New("must specify tasks"))
	}

	for _, info := range opts.Tasks {
		catcher.Add(info.validate())
	}

	for _, regexString := range opts.RegexFilters {
		_, err := regexp.Compile(regexString)
		catcher.Add(errors.Wrap(err, "compiling regex"))
	}

	return catcher.Resolve()
}

// TaskInfo specifies a set of test results to find.
type TaskInfo struct {
	TaskID      string `json:"task_id"`
	Execution   int    `json:"execution"`
	DisplayTask bool   `json:"display_task"`
}

func (info TaskInfo) validate() error {
	if info.TaskID == "" {
		return errors.New("must provide a task id")
	}
	if info.Execution < 0 {
		return errors.New("execution must be non-negative")
	}

	return nil
}

// Validate ensures GetSampleOptions is configured correctly.
func (opts GetSampleOptions) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(opts.Cedar.Validate())
	catcher.Add(opts.SampleOptions.validate())

	return catcher.Resolve()
}

func (opts GetSampleOptions) parse() (string, []byte, error) {
	urlString := fmt.Sprintf("%s/rest/v1/test_results/filtered_samples", opts.Cedar.BaseURL)
	req, err := json.Marshal(opts.SampleOptions)
	if err != nil {
		return "", nil, errors.Wrap(err, "marshaling sample options")
	}

	return urlString, req, nil
}

// GetSamples returns the filtered samples requested via HTTP to a Cedar service.
func GetSamples(ctx context.Context, opts GetSampleOptions) ([]byte, error) {
	resp, err := makeSamplesRequest(ctx, opts)
	if err != nil {
		return nil, err
	}

	catcher := grip.NewBasicCatcher()
	data, err := io.ReadAll(resp.Body)
	catcher.Wrap(err, "reading response body")
	catcher.Wrap(resp.Body.Close(), "closing response body")

	return data, catcher.Resolve()
}

func makeSamplesRequest(ctx context.Context, opts GetSampleOptions) (*http.Response, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	url, req, err := opts.parse()
	if err != nil {
		return nil, errors.Wrap(err, "parsing arguments")
	}

	resp, err := opts.Cedar.DoReq(ctx, url, bytes.NewBuffer(req))
	if err != nil {
		return nil, errors.Wrap(err, "requesting filtered samples from cedar")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to fetch filtered samples with resp '%s'", resp.Status)
	}

	return resp, nil
}
