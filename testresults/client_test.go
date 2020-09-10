package testresults

import (
	"context"
	"testing"

	"github.com/evergreen-ci/timber"
	"github.com/evergreen-ci/timber/testutil"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const basePort = 3000

func TestNewClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := testutil.NewMockMetricsServer(ctx, testutil.GetPortNumber(basePort))
	require.NoError(t, err)

	t.Run("Succeeds", func(t *testing.T) {
		httpClient := utility.GetHTTPClient()
		defer utility.PutHTTPClient(httpClient)
		client, err := NewClient(ctx, timber.ConnectionOptions{
			Client:   *httpClient,
			DialOpts: srv.DialOpts,
		})
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
	t.Run("FailsWithInvalidOptions", func(t *testing.T) {
		client, err := NewClient(ctx, timber.ConnectionOptions{})
		assert.Error(t, err)
		assert.Zero(t, client)
	})

	t.Run("WithExistingConnection", func(t *testing.T) {
		t.Run("Succeeds", func(t *testing.T) {
			conn, err := grpc.DialContext(ctx, srv.Address(), grpc.WithInsecure())
			require.NoError(t, err)
			c, err := NewClientWithExistingConnection(ctx, conn)
			require.NoError(t, err)
			assert.NotZero(t, c)
		})
		t.Run("FailsWithoutClient", func(t *testing.T) {
			c, err := NewClientWithExistingConnection(ctx, nil)
			assert.Error(t, err)
			assert.Zero(t, c)
		})
	})
}

func TestClient(t *testing.T) {

}
