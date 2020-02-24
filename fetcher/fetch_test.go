package fetcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Run("NoBaseURL", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL: "https://cedar.mongodb.com",
			TaskID:  "task",
		}
		_, err := opts.parse()
		require.NoError(t, err)
		opts.BaseURL = ""
		_, err = opts.parse()
		assert.Error(t, err)
	})
	t.Run("NoIDAndNoTaskID", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL: "https://cedar.mongodb.com",
			TaskID:  "task",
		}
		_, err := opts.parse()
		require.NoError(t, err)
		opts.TaskID = ""
		_, err = opts.parse()
		assert.Error(t, err)
	})
	t.Run("IDAndTaskID", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL: "https://cedar.mongodb.com",
			TaskID:  "task",
		}
		_, err := opts.parse()
		require.NoError(t, err)
		opts.ID = "id"
		_, err = opts.parse()
		assert.Error(t, err)
	})
	t.Run("ID", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL: "https://cedar.mongodb.com",
			ID:      "id",
		}
		url, err := opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/%s%s", opts.BaseURL, opts.ID, getParams(opts)), url)

		// meta
		opts.Meta = true
		url, err = opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/%s/meta%s", opts.BaseURL, opts.ID, getParams(opts)), url)
	})
	t.Run("TaskID", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL: "https://cedar.mongodb.com",
			TaskID:  "task",
		}
		url, err := opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/task_id/%s%s", opts.BaseURL, opts.TaskID, getParams(opts)), url)

		// meta
		opts.Meta = true
		url, err = opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/task_id/%s/meta%s", opts.BaseURL, opts.TaskID, getParams(opts)), url)
	})
	t.Run("TestName", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL:  "https://cedar.mongodb.com",
			TaskID:   "task",
			TestName: "test",
		}
		url, err := opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/test_name/%s/%s%s", opts.BaseURL, opts.TaskID, opts.TestName, getParams(opts)), url)

		// meta
		opts.Meta = true
		url, err = opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/test_name/%s/%s/meta%s", opts.BaseURL, opts.TaskID, opts.TestName, getParams(opts)), url)
	})
	t.Run("GroupID", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL:  "https://cedar.mongodb.com",
			TaskID:   "task",
			TestName: "test",
			GroupID:  "group",
		}
		url, err := opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/test_name/%s/%s/group/%s%s", opts.BaseURL, opts.TaskID, opts.TestName, opts.GroupID, getParams(opts)), url)

		// meta
		opts.Meta = true
		url, err = opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/test_name/%s/%s/group/%s/meta%s", opts.BaseURL, opts.TaskID, opts.TestName, opts.GroupID, getParams(opts)), url)
	})
	t.Run("Parameters", func(t *testing.T) {
		opts := FetchOptions{
			BaseURL:       "https://cedar.mongodb.com",
			TaskID:        "task",
			Start:         time.Now().Add(-time.Hour),
			End:           time.Now(),
			ProcessName:   "proc",
			Tags:          []string{"tag1", "tag2", "tag3"},
			PrintTime:     true,
			PrintPriority: true,
			Tail:          100,
			Limit:         1000,
		}
		url, err := opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/task_id/%s%s", opts.BaseURL, opts.TaskID, getParams(opts)), url)

		// meta
		opts.Meta = true
		url, err = opts.parse()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/task_id/%s/meta%s", opts.BaseURL, opts.TaskID, getParams(opts)), url)
	})
}

func getParams(opts FetchOptions) string {
	params := fmt.Sprintf(
		"?execution=%d&proc_name=%s&print_time=%v&print_priority=%v&n=%d&limit=%d&start=%s&end=%s",
		opts.Execution,
		opts.ProcessName,
		opts.PrintTime,
		opts.PrintPriority,
		opts.Tail,
		opts.Limit,
		opts.Start.Format(time.RFC3339),
		opts.End.Format(time.RFC3339),
	)
	for _, tag := range opts.Tags {
		params += fmt.Sprintf("&tags=%s", tag)
	}

	return params
}
