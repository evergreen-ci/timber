package testresults

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSampleOptionsValidate(t *testing.T) {
	for testName, testCase := range map[string]struct {
		name   string
		opts   FailedTestSampleOptions
		hasErr bool
	}{
		"NoTasks": {
			opts:   FailedTestSampleOptions{},
			hasErr: true,
		},
		"TaskWithoutID": {
			opts: FailedTestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: ""},
				},
			},
			hasErr: true,
		},
		"InvalidRegex": {
			opts: FailedTestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
				RegexFilters: []string{`[`},
			},
			hasErr: true,
		},
		"ValidWithRegex": {
			opts: FailedTestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
				RegexFilters: []string{`.*`},
			},
			hasErr: false,
		},
		"ValidWithoutRegex": {
			opts: FailedTestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
			},
			hasErr: false,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			err := testCase.opts.validate()
			if testCase.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSampleOptionsParse(t *testing.T) {
	for testName, testCase := range map[string]struct {
		opts            FailedTestSampleOptions
		expectedRequest string
		hasErr          bool
	}{
		"NoRegexes": {
			opts: FailedTestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
			},
			expectedRequest: `{"tasks":[{"task_id":"t1","execution":0,"display_task":false}]}`,
		},
		"WithRegexes": {
			opts: FailedTestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
				RegexFilters: []string{`.*`},
			},
			expectedRequest: `{"tasks":[{"task_id":"t1","execution":0,"display_task":false}],"regex_filters":[".*"]}`,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			opts := GetFailedSampleOptions{SampleOptions: testCase.opts}
			_, req, err := opts.parse()
			if testCase.hasErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, testCase.expectedRequest, string(req))
			}
		})
	}
}
