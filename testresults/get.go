package testresults

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// Valid sort by keys.
const (
	SortByStart      = "start"
	SortByDuration   = "duration"
	SortByTestName   = "test_name"
	SortByStatus     = "status"
	SortByBaseStatus = "base_status"
)

// GetOptions specify the required and optional information to create the test
// results HTTP GET request to Cedar.
type GetOptions struct {
	Cedar timber.GetOptions

	// Request information. See Cedar's REST documentation for more
	// information:
	// `https://github.com/evergreen-ci/cedar/wiki/Rest-V1-Usage`.
	Tasks        []TaskOptions
	Filter       *FilterOptions
	FailedSample bool
	Stats        bool
}

// TaskOptions specify the information required to fetch test results by task.
type TaskOptions struct {
	TaskID    string `json:"task_id"`
	Execution int    `json:"execution"`
}

// SortBy describes the properties by which to sort a set of test results.
type SortBy struct {
	Key      string `json:"key"`
	OrderDSC bool   `json:"order_dsc"`
}

// FilterOptions represent the parameters for filtering, sorting, and
// paginating test results. These options are only supported on select routes.
type FilterOptions struct {
	TestName  string        `json:"test_name,omitempty"`
	Statuses  []string      `json:"statuses,omitempty"`
	GroupID   string        `json:"group_id,omitempty"`
	Sort      []SortBy      `json:"sort"`
	Limit     int           `json:"limit,omitempty"`
	Page      int           `json:"page,omitempty"`
	BaseTasks []TaskOptions `json:"base_tasks,omitempty"`
}

type requestPayload struct {
	Tasks  []TaskOptions  `json:"tasks"`
	Filter *FilterOptions `json:"filter,omitempty"`
}

// Validate ensures TestResultsGetOptions is configured correctly.
func (opts GetOptions) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(opts.Cedar.Validate())
	catcher.NewWhen(len(opts.Tasks) == 0, "must specify at least one task")
	catcher.NewWhen(opts.FailedSample && opts.Stats, "cannot request the failed sample and stats, must be one or the other")
	catcher.NewWhen((opts.FailedSample || opts.Stats) && opts.Filter != nil, "cannot specify filter options on the failed_sample and stats routes")

	return catcher.Resolve()
}

func (opts GetOptions) serialize() (string, []byte, error) {
	urlString := fmt.Sprintf("%s/rest/v1/test_results/tasks", opts.Cedar.BaseURL)
	if opts.FailedSample {
		urlString += "/failed_sample"
	}
	if opts.Stats {
		urlString += "/stats"
	}

	payload := struct {
		Tasks  []TaskOptions  `json:"tasks"`
		Filter *FilterOptions `json:"filter,omitempty"`
	}{
		Tasks:  opts.Tasks,
		Filter: opts.Filter,
	}
	data, err := json.Marshal(&payload)
	if err != nil {
		return "", nil, errors.Wrap(err, "marshalling JSON request payload")
	}

	return urlString, data, nil
}

// Get returns the test results requested via HTTP to a Cedar service along
// with the status code of the request.
func Get(ctx context.Context, opts GetOptions) ([]byte, int, error) {
	if err := opts.Validate(); err != nil {
		return nil, 0, errors.WithStack(err)
	}

	url, payload, err := opts.serialize()
	if err != nil {
		return nil, 0, errors.Wrap(err, "serializing request options")
	}

	resp, err := opts.Cedar.DoReq(ctx, url, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, errors.Wrap(err, "requesting test results from cedar")
	}

	catcher := grip.NewBasicCatcher()
	data, err := io.ReadAll(resp.Body)
	catcher.Wrap(err, "reading response body")
	catcher.Wrap(resp.Body.Close(), "closing response body")

	return data, resp.StatusCode, catcher.Resolve()
}
