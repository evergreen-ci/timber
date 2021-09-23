package buildlogger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/peterhellberg/link"
	"github.com/pkg/errors"
)

// GetOptions specify the required and optional information to create the
// buildlogger HTTP GET request to cedar.
type GetOptions struct {
	CedarOpts timber.GetOptions

	// Request information. See cedar's REST documentation for more
	// information:
	// `https://github.com/evergreen-ci/cedar/wiki/Rest-V1-Usage`.
	ID       string
	TaskID   string
	TestName string
	GroupID  string
	Meta     bool

	// Query parameters.
	Execution     *int
	Start         time.Time
	End           time.Time
	ProcessName   string
	Tags          []string
	PrintTime     bool
	PrintPriority bool
	Tail          int
	Limit         int
}

// Validate ensures BuildloggerGetOptions is configured correctly.
func (opts GetOptions) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(opts.CedarOpts.Validate())
	catcher.AddWhen(opts.ID == "" && opts.TaskID == "", errors.New("must provide an id or task id"))
	catcher.AddWhen(opts.ID != "" && opts.TaskID != "", errors.New("cannot provide both id and task id"))
	catcher.AddWhen(opts.TestName != "" && opts.TaskID == "", errors.New("must provide a task id when a test name is specified"))
	catcher.AddWhen(opts.GroupID != "" && opts.TaskID == "", errors.New("must provide a task id when a group id is specified"))
	catcher.AddWhen(opts.GroupID != "" && opts.Meta, errors.New("cannot specify a group id and set meta to true"))

	return catcher.Resolve()
}

func (opts GetOptions) parse() string {
	params := []string{}
	if opts.Execution != nil {
		params = append(params, fmt.Sprintf("execution=%d", *opts.Execution))
	}
	if !opts.Start.IsZero() {
		params = append(params, fmt.Sprintf("start=%s", opts.Start.Format(time.RFC3339)))
	}
	if !opts.End.IsZero() {
		params = append(params, fmt.Sprintf("end=%s", opts.End.Format(time.RFC3339)))
	}
	if opts.ProcessName != "" && !opts.Meta {
		params = append(params, fmt.Sprintf("proc_name=%s", url.QueryEscape(opts.ProcessName)))
	}
	for _, tag := range opts.Tags {
		params = append(params, fmt.Sprintf("tags=%s", url.QueryEscape(tag)))
	}
	if opts.PrintTime && !opts.Meta {
		params = append(params, "print_time=true")
	}
	if opts.PrintPriority && !opts.Meta {
		params = append(params, "print_priority=true")
	}
	if opts.Tail > 0 && !opts.Meta {
		params = append(params, fmt.Sprintf("n=%d", opts.Tail))
	}
	if opts.Limit > 0 && !opts.Meta {
		params = append(params, fmt.Sprintf("limit=%d", opts.Limit))
	}
	if opts.Limit <= 0 && opts.Tail <= 0 && !opts.Meta {
		params = append(params, "paginate=true")
	}

	urlString := fmt.Sprintf("%s/rest/v1/buildlogger", opts.CedarOpts.BaseURL)
	if opts.ID != "" {
		urlString += fmt.Sprintf("/%s", url.PathEscape(opts.ID))
	} else if opts.TestName != "" {
		urlString += fmt.Sprintf("/test_name/%s/%s", url.PathEscape(opts.TaskID), url.PathEscape(opts.TestName))
	} else {
		urlString += fmt.Sprintf("/task_id/%s", url.PathEscape(opts.TaskID))
	}
	if opts.GroupID != "" {
		urlString += fmt.Sprintf("/group/%s", url.PathEscape(opts.GroupID))
	} else if opts.Meta {
		urlString += "/meta"
	}
	if len(params) > 0 {
		urlString += "?" + strings.Join(params, "&")
	}

	return urlString
}

// Get returns a ReadCloser with the logs or log metadata requested via HTTP to
// a cedar service.
func Get(ctx context.Context, opts GetOptions) (io.ReadCloser, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := opts.CedarOpts.DoReq(ctx, opts.parse())
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			return &paginatedReadCloser{
				ctx:        ctx,
				header:     resp.Header,
				opts:       opts.CedarOpts,
				ReadCloser: resp.Body,
			}, nil
		}
		return nil, errors.Errorf("failed to fetch logs with resp '%s'", resp.Status)
	}
	return nil, errors.Wrapf(err, "fetch logs request failed")
}

type paginatedReadCloser struct {
	ctx    context.Context
	header http.Header
	opts   timber.GetOptions

	io.ReadCloser
}

func (r *paginatedReadCloser) Read(p []byte) (int, error) {
	if r.ReadCloser == nil {
		return 0, io.EOF
	}

	var (
		n      int
		offset int
		err    error
	)
	for offset < len(p) {
		n, err = r.ReadCloser.Read(p[offset:])
		offset += n
		if err == io.EOF {
			err = r.getNextPage()
		}
		if err != nil {
			break
		}
	}

	return offset, err
}

func (r *paginatedReadCloser) getNextPage() error {
	group, ok := link.ParseHeader(r.header)["next"]
	if ok {
		resp, err := r.opts.DoReq(r.ctx, group.URI)
		if err != nil {
			return errors.Wrap(err, "requesting next page")
		}

		if err = r.Close(); err != nil {
			return errors.Wrap(err, "closing last response reader")
		}

		r.header = resp.Header
		r.ReadCloser = resp.Body
	} else {
		return io.EOF
	}

	return nil
}
