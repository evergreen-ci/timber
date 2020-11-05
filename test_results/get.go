package test_results

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// GetTestResults returns with the test results requested via HTTP to a cedar
// service.
func GetTestResults(ctx context.Context, opts timber.GetOptions) ([]byte, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	url := fmt.Sprintf("%s/rest/v1/testresults/test_name/%s", opts.BaseURL, opts.TaskID)
	if opts.TestName != "" {
		url += fmt.Sprintf("/%s", opts.TestName)
	}

	resp, err := opts.DoReq(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "problem requesting test results from cedar")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to fetch test results with resp '%s'", resp.Status)
	}

	catcher := grip.NewBasicCatcher()
	data, err := ioutil.ReadAll(resp.Body)
	catcher.Add(err)
	catcher.Add(resp.Body.Close())

	return data, catcher.Resolve()
}