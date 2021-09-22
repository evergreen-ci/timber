package testresults

import (
	"testing"

	"github.com/evergreen-ci/timber"
	"github.com/stretchr/testify/assert"
)

func TestGetOptionsValidate(t *testing.T) {
	for _, test := range []struct {
		name   string
		opts   GetOptions
		hasErr bool
	}{
		{
			name: "InvalidCedarOpts",
			opts: GetOptions{
				TaskID: "task",
			},
			hasErr: true,
		},
		{
			name: "MissingTaskID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
			},
			hasErr: true,
		},
		{
			name: "FailedSampleAndStats",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:       "task",
				FailedSample: true,
				Stats:        true,
			},
			hasErr: true,
		},
		{
			name: "TaskID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID: "task",
			},
		},
		{
			name: "TaskIDAndFailedSample",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:       "task",
				FailedSample: true,
			},
		},
		{
			name: "TaskIDAndStats",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:       "task",
				FailedSample: true,
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

func TestGetURL(t *testing.T) {
	cedarOpts := timber.GetOptions{BaseURL: "https://url.com"}
	baseURL := cedarOpts.BaseURL + "/rest/v1/test_results"
	exec := 1
	for _, test := range []struct {
		name        string
		opts        GetOptions
		expectedURL string
	}{
		{
			name: "TaskID",
			opts: GetOptions{
				CedarOpts: cedarOpts,
				TaskID:    "task",
			},
			expectedURL: baseURL + "/task_id/task",
		},
		{
			name: "TaskIDWithParams",
			opts: GetOptions{
				CedarOpts:    cedarOpts,
				TaskID:       "task",
				Execution:    &exec,
				DisplayTask:  true,
				TestName:     "test",
				Statuses:     []string{"fail", "silentfail"},
				GroupID:      "group",
				SortBy:       "sort",
				SortOrderDSC: true,
				BaseTaskID:   "base_task",
				Limit:        100,
				Page:         5,
			},
			expectedURL: baseURL + "/task_id/task?execution=1&display_task=true&test_name=test&status=fail&status=silentfail&group_id=group&sort_by=sort&sort_order_dsc=true&base_task_id=base_task&limit=100&page=5",
		},
		{
			name: "FailedSample",
			opts: GetOptions{
				CedarOpts:    cedarOpts,
				TaskID:       "task",
				FailedSample: true,
			},
			expectedURL: baseURL + "/task_id/task/failed_sample",
		},
		{
			name: "Stats",
			opts: GetOptions{
				CedarOpts: cedarOpts,
				TaskID:    "task",
				Stats:     true,
			},
			expectedURL: baseURL + "/task_id/task/stats",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedURL, test.opts.getURL())
		})
	}
}
