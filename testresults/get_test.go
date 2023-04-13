package testresults

import (
	"encoding/json"
	"testing"

	"github.com/evergreen-ci/timber"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
			},
			hasErr: true,
		},
		{
			name: "EmptyTasks",
			opts: GetOptions{
				Cedar: timber.GetOptions{
					BaseURL: "https://url.com",
				},
			},
			hasErr: true,
		},
		{
			name: "FailedSampleAndStats",
			opts: GetOptions{
				Cedar: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				FailedSample: true,
				Stats:        true,
			},
			hasErr: true,
		},
		{
			name: "FailedSampleAndFilterOpts",
			opts: GetOptions{
				Cedar: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				Filter:       &FilterOptions{},
				FailedSample: true,
			},
			hasErr: true,
		},
		{
			name: "StatsAndFilterOpts",
			opts: GetOptions{
				Cedar: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				Filter: &FilterOptions{},
				Stats:  true,
			},
			hasErr: true,
		},
		{
			name: "OnlyTasks",
			opts: GetOptions{
				Cedar: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
			},
		},
		{
			name: "TasksAndFailedSample",
			opts: GetOptions{
				Cedar: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				FailedSample: true,
			},
		},
		{
			name: "TasksAndStats",
			opts: GetOptions{
				Cedar: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				Stats: true,
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

func TestParse(t *testing.T) {
	cedarOpts := timber.GetOptions{BaseURL: "https://url.com"}
	baseURL := cedarOpts.BaseURL + "/rest/v1/test_results"
	for _, test := range []struct {
		name        string
		opts        GetOptions
		expectedURL string
	}{
		{
			name: "OnlyTasks",
			opts: GetOptions{
				Cedar: cedarOpts,
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
			},
			expectedURL: baseURL + "/tasks",
		},
		{
			name: "TasksAndFilter",
			opts: GetOptions{
				Cedar: cedarOpts,
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				Filter: &FilterOptions{
					TestName: "test",
					Statuses: []string{"fail", "silentfail"},
					GroupID:  "group",
					Sort: []SortBy{
						{
							Key: "key0",
						},
						{
							Key:      "key1",
							OrderDSC: true,
						},
					},
					Limit: 100,
					Page:  5,
					BaseTasks: []TaskOptions{
						{
							TaskID:    "base_task",
							Execution: 1,
						},
					},
				},
			},
			expectedURL: baseURL + "/tasks",
		},
		{
			name: "FailedSample",
			opts: GetOptions{
				Cedar: cedarOpts,
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				FailedSample: true,
			},
			expectedURL: baseURL + "/tasks/failed_sample",
		},
		{
			name: "Stats",
			opts: GetOptions{
				Cedar: cedarOpts,
				Tasks: []TaskOptions{
					{
						TaskID:    "task",
						Execution: 0,
					},
				},
				Stats: true,
			},
			expectedURL: baseURL + "/tasks/stats",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			urlString, payload, err := test.opts.serialize()
			require.NoError(t, err)

			assert.Equal(t, test.expectedURL, urlString)
			expectedPayload, err := json.Marshal(&struct {
				Tasks  []TaskOptions  `json:"tasks"`
				Filter *FilterOptions `json:"filter,omitempty"`
			}{
				Tasks:  test.opts.Tasks,
				Filter: test.opts.Filter,
			})
			require.NoError(t, err)
			assert.Equal(t, expectedPayload, payload)
		})
	}
}
