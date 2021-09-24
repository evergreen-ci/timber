package timber

import (
	"context"
	"io"
	"net/http"

	"github.com/peterhellberg/link"
	"github.com/pkg/errors"
)

// paginatedReadCloser implements the io.ReadCloser interface, wrapping the
// io.ReadCloser from an HTTP response. The reader will first exhaust the
// underlying reader, then, using the HTTP response headers, request the next
// page, if any, and replace the underlying reader. An io.EOF error is returned
// when either the underlying reader is exhausted and the HTTP response header
// does not contain a "next" field, or the "next" field's URL returns no data.
type paginatedReadCloser struct {
	ctx    context.Context
	header http.Header
	opts   GetOptions

	io.ReadCloser
}

// NewPaginatedReadCloser returns a new paginated read closer with the body and
// header of the given HTTP response. The GetOptions are used to make any
// subsequent page requests to the Cedar service.
func NewPaginatedReadCloser(ctx context.Context, resp *http.Response, opts GetOptions) io.ReadCloser {
	return &paginatedReadCloser{
		ctx:        ctx,
		header:     resp.Header,
		opts:       opts,
		ReadCloser: resp.Body,
	}
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
