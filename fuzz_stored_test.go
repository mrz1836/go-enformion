package enformion_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

func FuzzDecodeSearchPersonsMatchesTheCall(f *testing.F) {
	storedFuzz(f, storedSearch(), storedSearchBodies(f))
}

func FuzzDecodeEnrichContactMatchesTheCall(f *testing.F) {
	storedFuzz(f, storedEnrich(), storedEnrichBodies(f))
}

// storedFuzz seeds f with seeds and checks, for each body, that decoding
// leaves the body unchanged, gives the same outcome twice, fails only as an
// HTTP 200 response can and without diagnostics, and gives the outcome of the
// call through an in-memory transport whose limit is the decoders' bound.
func storedFuzz[R any](f *testing.F, op storedOperation[R], seeds []storedBody) {
	for _, s := range seeds {
		f.Add(s.body)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		kept := bytes.Clone(body)
		res, err := op.decode(body)
		require.Equal(t, kept, body, "the body is never modified")
		again, againErr := op.decode(body)
		requireSameOutcome(t, res, err, again, againErr)
		if err != nil {
			ce := callError(t, err)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, enformion.ProviderDiagnostics{}, ce.Diagnostics())
			assert.Contains(t, []enformion.FailureKind{
				enformion.FailureMalformedResponse, enformion.FailureUnexpectedResponse,
				enformion.FailureRejected, enformion.FailureResponseTooLarge,
			}, ce.Kind)
		}

		rt := testutil.Transport(http.StatusOK, int64(len(body)), func() io.ReadCloser {
			return testutil.NewBody(string(body))
		})
		c := newTransportClient(t, rt, enformion.WithMaxResponseBytes(storedLimit))
		want, wantErr := op.call(t, c)
		requireSameOutcome(t, want, wantErr, res, err)
	})
}
