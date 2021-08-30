package testresults

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// TestResultsGetOptions specify the required and optional information to
// create the test results HTTP GET request to cedar.
type TestResultsGetOptions struct {
	CedarOpts timber.GetOptions

	// Request information. See cedar's REST documentation for more
	// information:
	// `https://github.com/evergreen-ci/cedar/wiki/Rest-V1-Usage`.
	TaskID       string
	TestName     string
	Execution    *int
	FailedSample bool
	Stats        bool
	DisplayTask  bool
}

// Validate ensures TestResultsGetOptions is configured correctly.
func (opts *TestResultsGetOptions) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(opts.CedarOpts.Validate())
	catcher.NewWhen(opts.TaskID == "", "must provide a task id")
	catcher.NewWhen(opts.FailedSample && opts.TestName != "", "cannot request the failed sample when requesting a single test result")
	catcher.NewWhen(opts.FailedSample && opts.Stats, "cannot request the failed sample and stats, must be one or the other")

	return catcher.Resolve()
}

// GetTestResults returns with the test results requested via HTTP to a cedar
// service.
func GetTestResults(ctx context.Context, opts TestResultsGetOptions) ([]byte, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	urlString := fmt.Sprintf("%s/rest/v1/test_results/task_id/%s", opts.CedarOpts.BaseURL, url.PathEscape(opts.TaskID))
	if opts.FailedSample {
		urlString += "/failed_sample"
	}
	if opts.Stats {
		urlString += "/stats"
	}

	var params string
	if opts.TestName != "" {
		params += fmt.Sprintf("test_name=%s", opts.TestName)
	}
	if opts.Execution != nil {
		params += fmt.Sprintf("execution=%d", opts.Execution)
	}
	if opts.DisplayTask {
		params += "display_task=true"
	}
	if len(params) > 0 {
		urlString += "?" + params
	}

	catcher := grip.NewBasicCatcher()
	resp, err := opts.CedarOpts.DoReq(ctx, urlString)
	if err != nil {
		return nil, errors.Wrap(err, "requesting test results from cedar")
	}
	if resp.StatusCode != http.StatusOK {
		catcher.Add(resp.Body.Close())
		catcher.Add(errors.Errorf("failed to fetch test results with resp '%s'", resp.Status))
		return nil, catcher.Resolve()
	}

	data, err := ioutil.ReadAll(resp.Body)
	catcher.Add(err)
	catcher.Add(resp.Body.Close())

	return data, catcher.Resolve()
}
