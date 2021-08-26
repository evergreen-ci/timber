package testresults

import (
	"testing"

	"github.com/evergreen-ci/timber"
	"github.com/stretchr/testify/assert"
)

func TestTestResultsGetOptionsValidate(t *testing.T) {
	for _, test := range []struct {
		name   string
		opts   TestResultsGetOptions
		hasErr bool
	}{
		{
			name: "FailsWithInvalidCedarOpts",
			opts: TestResultsGetOptions{
				TaskID: "task",
			},
			hasErr: true,
		},
		{
			name: "FailsWithMissingTaskID",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
			},
			hasErr: true,
		},
		{
			name: "FailsWithTestNameAndFailedSample",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:       "display",
				TestName:     "test",
				FailedSample: true,
			},
			hasErr: true,
		},
		{
			name: "SucceedsWithTaskID",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID: "task",
			},
		},
		{
			name: "SucceedsWithTaskIDAndFailedSample",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:       "task",
				FailedSample: true,
			},
		},
		{
			name: "SucceedsWithDisplayTaskID",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:      "display",
				DisplayTask: true,
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndFailedSample",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:       "display",
				FailedSample: true,
				DisplayTask:  true,
			},
		},
		{
			name: "SucceedsWithTaskIDAndTestName",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:   "task",
				TestName: "test",
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndTestName",
			opts: TestResultsGetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:      "task",
				TestName:    "test",
				DisplayTask: true,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.opts.Validate()
			if test.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
