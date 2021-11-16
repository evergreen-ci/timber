package testresults

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSampleOptionsValidate(t *testing.T) {
	for testName, testCase := range map[string]struct {
		name   string
		opts   TestSampleOptions
		hasErr bool
	}{
		"NoTasks": {
			opts:   TestSampleOptions{},
			hasErr: true,
		},
		"TaskWithoutID": {
			opts: TestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: ""},
				},
			},
			hasErr: true,
		},
		"InvalidRegex": {
			opts: TestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
				RegexFilters: []string{`[`},
			},
			hasErr: true,
		},
		"ValidWithRegex": {
			opts: TestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
				RegexFilters: []string{`.*`},
			},
			hasErr: false,
		},
		"ValidWithoutRegex": {
			opts: TestSampleOptions{
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
		opts            TestSampleOptions
		expectedRequest string
		hasErr          bool
	}{
		"NoRegexes": {
			opts: TestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
			},
			expectedRequest: `{"tasks":[{"task_id":"t1","execution":0,"display_task":false}]}`,
		},
		"WithRegexes": {
			opts: TestSampleOptions{
				Tasks: []TaskInfo{
					{TaskID: "t1"},
				},
				RegexFilters: []string{`.*`},
			},
			expectedRequest: `{"tasks":[{"task_id":"t1","execution":0,"display_task":false}],"regex_filters":[".*"]}`,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			opts := GetSampleOptions{SampleOptions: testCase.opts}
			_, req, err := opts.parse()
			if testCase.hasErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, testCase.expectedRequest, string(req))
			}
		})
	}
}
