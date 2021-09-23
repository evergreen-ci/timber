package buildlogger

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/evergreen-ci/timber"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
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
			name: "MissingIDAndTaskID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
			},
			hasErr: true,
		},
		{
			name: "IDAndTaskID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				ID:     "id",
				TaskID: "task",
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
			hasErr: true,
		},
		{
			name: "TestNameAndNoTaskID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				ID:       "id",
				TestName: "test",
			},
			hasErr: true,
		},
		{
			name: "GroupIDAndNoTaskID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				ID:      "id",
				GroupID: "group",
			},
			hasErr: true,
		},
		{
			name: "GroupIDAndMeta",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:  "task",
				GroupID: "group",
				Meta:    true,
			},
			hasErr: true,
		},
		{
			name: "ID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				ID: "id",
			},
		},
		{
			name: "IDAndMeta",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				ID: "id",
			},
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
			name: "TaskIDAndMeta",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID: "task",
				Meta:   true,
			},
		},
		{
			name: "TestName",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:   "task",
				TestName: "test",
				Meta:     true,
			},
		},
		{
			name: "TestNameAndMeta",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:   "task",
				TestName: "test",
				Meta:     true,
			},
		},
		{
			name: "TaskIDAndGroupID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:  "task",
				GroupID: "group",
			},
		},
		{
			name: "TestNameAndGroupID",
			opts: GetOptions{
				CedarOpts: timber.GetOptions{
					BaseURL: "https://url.com",
				},
				TaskID:   "task",
				TestName: "test",
				GroupID:  "group",
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
	t.Run("ID", func(t *testing.T) {
		opts := GetOptions{
			CedarOpts: timber.GetOptions{
				BaseURL: "https://cedar.mongodb.com",
			},
			ID: "id/1",
		}
		t.Run("Logs", func(t *testing.T) {
			assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/%s?paginate=true", opts.CedarOpts.BaseURL, url.PathEscape(opts.ID)), opts.parse())
		})
		t.Run("Meta", func(t *testing.T) {
			opts.Meta = true
			assert.Equal(t, fmt.Sprintf("%s/rest/v1/buildlogger/%s/meta%s", opts.CedarOpts.BaseURL, url.PathEscape(opts.ID)), opts.parse())
		})
	})
	t.Run("TaskID", func(t *testing.T) {
		opts := GetOptions{
			CedarOpts: timber.GetOptions{
				BaseURL: "https://cedar.mongodb.com",
			},
			TaskID:        "task?",
			Execution:     utility.ToIntPtr(3),
			Start:         time.Now().Add(-time.Hour),
			End:           time.Now(),
			ProcessName:   "proc/1",
			Tags:          []string{"tag1?", "tag/2", "tag3"},
			PrintTime:     true,
			PrintPriority: true,
			Tail:          100,
			Limit:         1000,
		}
		t.Run("Logs", func(t *testing.T) {
			expectedURL := fmt.Sprintf(
				"%s/rest/v1/buildlogger/task_id/%s?execution=3&start=%s&end=%s&proc_name=%s&tag=%s&tag=%s&tag=tag3&print_time=true&print_priority=true&tail=100&limit=100",
				opts.CedarOpts.BaseURL,
				url.PathEscape(opts.TaskID),
				opts.Start.Format(time.RFC3339),
				opts.End.Format(time.RFC3339),
				url.QueryEscape(opts.ProcessName),
				url.QueryEscape(opts.Tags[0]),
				url.QueryEscape(opts.Tags[1]),
			)
			assert.Equal(t, expectedURL, opts.parse())
		})
		t.Run("Meta", func(t *testing.T) {
			opts.Meta = true
			expectedURL := fmt.Sprintf(
				"%s/rest/v1/buildlogger/task_id/%s?execution=3&start=%s&end=%s&tag=%s&tag=%s&tag=tag3",
				opts.CedarOpts.BaseURL,
				url.PathEscape(opts.TaskID),
				opts.Start.Format(time.RFC3339),
				opts.End.Format(time.RFC3339),
				url.QueryEscape(opts.Tags[0]),
				url.QueryEscape(opts.Tags[1]),
			)
			assert.Equal(t, expectedURL, opts.parse())
		})
	})
	t.Run("TestName", func(t *testing.T) {
		opts := GetOptions{
			CedarOpts: timber.GetOptions{
				BaseURL: "https://cedar.mongodb.com",
			},
			TaskID:   "task?",
			TestName: "test/1",
		}
		t.Run("Logs", func(t *testing.T) {
			expectedURL := fmt.Sprintf(
				"%s/rest/v1/buildlogger/test_name/%s/%s?paginate=true",
				opts.CedarOpts.BaseURL,
				url.PathEscape(opts.TaskID),
				url.PathEscape(opts.TestName),
			)
			assert.Equal(t, expectedURL, opts.parse())
		})
		t.Run("Meta", func(t *testing.T) {
			opts.Meta = true
			expectedURL := fmt.Sprintf(
				"%s/rest/v1/buildlogger/test_name/%s/%s/meta",
				opts.CedarOpts.BaseURL,
				url.PathEscape(opts.TaskID),
				url.PathEscape(opts.TestName),
			)
			assert.Equal(t, expectedURL, opts.parse())
		})
	})
	t.Run("TaskIDAndGroupID", func(t *testing.T) {
		opts := GetOptions{
			CedarOpts: timber.GetOptions{
				BaseURL: "https://cedar.mongodb.com",
			},
			TaskID:  "task?",
			GroupID: "group/group/group",
		}
		expectedURL := fmt.Sprintf(
			"%s/rest/v1/buildlogger/task_id/%s/group/%s&paginate=true",
			opts.CedarOpts.BaseURL,
			url.PathEscape(opts.TaskID),
			url.PathEscape(opts.GroupID),
		)
		assert.Equal(t, expectedURL, opts.parse())
	})
	t.Run("TestNameAndGroupID", func(t *testing.T) {
		opts := GetOptions{
			CedarOpts: timber.GetOptions{
				BaseURL: "https://cedar.mongodb.com",
			},
			TaskID:   "task?",
			TestName: "test/?1",
			GroupID:  "group/group/group",
		}
		expectedURL := fmt.Sprintf(
			"%s/rest/v1/buildlogger/test_name/%s/%s/group/%s?paginate=true",
			opts.CedarOpts.BaseURL,
			url.PathEscape(opts.TaskID),
			url.PathEscape(opts.TestName),
			url.PathEscape(opts.GroupID),
		)
		assert.Equal(t, expectedURL, opts.parse())
	})
}

func TestPaginatedReadCloser(t *testing.T) {
	t.Run("PaginatedRoute", func(t *testing.T) {
		handler := &mockHandler{pages: 3}
		server := httptest.NewServer(handler)
		handler.baseURL = server.URL

		opts := timber.GetOptions{}
		resp, err := opts.DoReq(context.TODO(), server.URL)
		require.NoError(t, err)

		var r io.ReadCloser
		r = &paginatedReadCloser{
			ctx:        context.TODO(),
			header:     resp.Header,
			ReadCloser: resp.Body,
		}

		data, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, "PAGINATED BODY PAGE 1\nPAGINATED BODY PAGE 2\nPAGINATED BODY PAGE 3\n", string(data))
		assert.NoError(t, r.Close())
	})
	t.Run("NonPaginatedRoute", func(t *testing.T) {
		handler := &mockHandler{}
		server := httptest.NewServer(handler)
		handler.baseURL = server.URL

		opts := timber.GetOptions{}
		resp, err := opts.DoReq(context.TODO(), server.URL)
		require.NoError(t, err)

		var r io.ReadCloser
		r = &paginatedReadCloser{
			ctx:        context.TODO(),
			header:     resp.Header,
			ReadCloser: resp.Body,
		}

		data, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, "NON-PAGINATED BODY PAGE", string(data))
		assert.NoError(t, r.Close())
	})
	t.Run("SplitPageByteSlice", func(t *testing.T) {
		handler := &mockHandler{pages: 2}
		server := httptest.NewServer(handler)
		handler.baseURL = server.URL

		opts := timber.GetOptions{}
		resp, err := opts.DoReq(context.TODO(), server.URL)
		require.NoError(t, err)

		var r io.ReadCloser
		r = &paginatedReadCloser{
			ctx:        context.TODO(),
			header:     resp.Header,
			ReadCloser: resp.Body,
		}

		p := make([]byte, 33) // 1.5X len of each page
		n, err := r.Read(p)
		require.NoError(t, err)
		assert.Equal(t, len(p), n)
		assert.Equal(t, "PAGINATED BODY PAGE 1\nPAGINATED B", string(p))
		p = make([]byte, 33)
		n, err = r.Read(p)
		require.Equal(t, io.EOF, err)
		assert.Equal(t, 11, n)
		assert.Equal(t, "ODY PAGE 2\n", string(p[:11]))
		assert.NoError(t, r.Close())
	})
}

type mockHandler struct {
	baseURL string
	pages   int
	count   int
}

func (h *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.pages > 0 {
		if h.count <= h.pages-1 {
			w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"%s\"", h.baseURL, "next"))
			_, _ = w.Write([]byte(fmt.Sprintf("PAGINATED BODY PAGE %d\n", h.count+1)))
		}
		h.count++
	} else {
		_, _ = w.Write([]byte("NON-PAGINATED BODY PAGE"))
	}
}

func getParams(opts GetOptions) string {
	params := fmt.Sprintf(
		"?execution=%d&proc_name=%s&print_time=%v&print_priority=%v&n=%d&limit=%d&paginate=true",
		opts.Execution,
		url.QueryEscape(opts.ProcessName),
		opts.PrintTime,
		opts.PrintPriority,
		opts.Tail,
		opts.Limit,
	)
	if !opts.Start.IsZero() {
		params += fmt.Sprintf("&start=%s", opts.Start.Format(time.RFC3339))
	}
	if !opts.End.IsZero() {
		params += fmt.Sprintf("&end=%s", opts.End.Format(time.RFC3339))
	}
	for _, tag := range opts.Tags {
		params += fmt.Sprintf("&tags=%s", tag)
	}

	return params
}
