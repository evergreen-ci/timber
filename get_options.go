package timber

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

// GetOptions specify the required and optional information to create an HTTP
// GET request to Cedar.
type GetOptions struct {
	// The Cedar service's base HTTP URL for the request.
	BaseURL string
	// The user cookie for Cedar authorization. Optional.
	Cookie *http.Cookie
	// User API key and name for request header.
	UserKey  string
	UserName string
	// HTTP client for connecting to the Cedar service. Optional.
	HTTPClient *http.Client
}

// Validate ensures GetOptions is configured correctly.
func (opts GetOptions) Validate() error {
	if opts.BaseURL == "" {
		return errors.New("must provide a base URL")
	}

	return nil
}

// DoReq makes an HTTP request to the Cedar service.
func (opts GetOptions) DoReq(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, body)
	if err != nil {
		return nil, errors.Wrap(err, "creating http request for Cedar")
	}
	if opts.Cookie != nil {
		req.AddCookie(opts.Cookie)
	}
	if opts.UserKey != "" && opts.UserName != "" {
		req.Header.Set("Evergreen-Api-Key", opts.UserKey)
		req.Header.Set("Evergreen-Api-User", opts.UserName)
	}

	c := opts.HTTPClient
	fmt.Println(c)
	if c == nil {
		c = utility.GetHTTPClient()
		defer utility.PutHTTPClient(c)
	}

	return c.Do(req)
}
