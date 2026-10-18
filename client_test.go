package enformion_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// ctMarker prefixes every rejected value, so a test can prove it never appears
// in an error.
const ctMarker = "CTMARKER"

func TestClientCredentialValidation(t *testing.T) {
	long := ctMarker + strings.Repeat("k", 1024-len(ctMarker))
	tests := []struct {
		name   string
		value  string
		reason string
	}{
		{name: "empty", value: "", reason: "required"},
		{name: "1025 bytes", value: long + "k", reason: "invalid_header_value"},
		{name: "carriage return", value: ctMarker + "\rx", reason: "invalid_header_value"},
		{name: "line feed", value: ctMarker + "\nx", reason: "invalid_header_value"},
		{name: "NUL", value: ctMarker + "\x00x", reason: "invalid_header_value"},
		{name: "tab", value: ctMarker + "\tx", reason: "invalid_header_value"},
		{name: "DEL", value: ctMarker + "\x7fx", reason: "invalid_header_value"},
		{name: "non-ASCII", value: ctMarker + "éx", reason: "invalid_header_value"},
		{name: "leading space", value: " " + ctMarker, reason: "invalid_header_value"},
		{name: "trailing space", value: ctMarker + " ", reason: "invalid_header_value"},
	}
	for _, field := range []string{"keyName", "keyPassword"} {
		for _, tt := range tests {
			t.Run(field+"/"+tt.name, func(t *testing.T) {
				name, password := testKeyName, testKeyPassword
				if field == "keyName" {
					name = tt.value
				} else {
					password = tt.value
				}
				c, err := enformion.NewClient(name, password)
				require.Error(t, err)
				assert.Nil(t, c)
				require.ErrorIs(t, err, enformion.ErrInvalidConfig)
				ve := validationError(t, err)
				assert.Equal(t, field, ve.Field)
				assert.Equal(t, tt.reason, ve.Reason)
				assert.Equal(t, "enformion: invalid configuration: "+field+": "+tt.reason, err.Error())
				if tt.value != "" {
					testutil.AssertRedacted(t, err.Error(), []string{ctMarker})
				}
			})
		}
	}
}

func TestClientCredentialAccepted(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "one byte", value: "k"},
		{name: "inner spaces", value: "key name with spaces"},
		{name: "every printable ASCII byte", value: "!\"#$%&'()*+,-./0123456789:;<=>?@AZ[\\]^_`az{|}~"},
		{name: "exactly 1024 bytes", value: strings.Repeat("k", 1024)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := enformion.NewClient(tt.value, tt.value)
			require.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

func TestClientOptionValidation(t *testing.T) {
	tests := []struct {
		name   string
		opts   []enformion.Option
		field  string
		reason string
	}{
		{name: "nil HTTP client", opts: []enformion.Option{enformion.WithHTTPClient(nil)}, field: "HTTPClient", reason: "nil"},
		{name: "zero timeout", opts: []enformion.Option{enformion.WithTimeout(0)}, field: "Timeout", reason: "out_of_range"},
		{name: "negative timeout", opts: []enformion.Option{enformion.WithTimeout(-time.Second)}, field: "Timeout", reason: "out_of_range"},
		{name: "timeout over 5m", opts: []enformion.Option{enformion.WithTimeout(5*time.Minute + time.Nanosecond)}, field: "Timeout", reason: "out_of_range"},
		{name: "zero max bytes", opts: []enformion.Option{enformion.WithMaxResponseBytes(0)}, field: "MaxResponseBytes", reason: "out_of_range"},
		{name: "negative max bytes", opts: []enformion.Option{enformion.WithMaxResponseBytes(-1)}, field: "MaxResponseBytes", reason: "out_of_range"},
		{name: "max bytes over 16 MiB", opts: []enformion.Option{enformion.WithMaxResponseBytes(16<<20 + 1)}, field: "MaxResponseBytes", reason: "out_of_range"},
		{name: "invalid test URL", opts: []enformion.Option{enformion.WithTestBaseURL("http://" + ctMarker)}, field: "TestBaseURL", reason: "invalid_url"},
		{name: "nil option is skipped", opts: []enformion.Option{nil, enformion.WithTimeout(0)}, field: "Timeout", reason: "out_of_range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := enformion.NewClient(testKeyName, testKeyPassword, tt.opts...)
			require.Error(t, err)
			assert.Nil(t, c)
			require.ErrorIs(t, err, enformion.ErrInvalidConfig)
			ve := validationError(t, err)
			assert.Equal(t, tt.field, ve.Field)
			assert.Equal(t, tt.reason, ve.Reason)
			testutil.AssertRedacted(t, err.Error(), []string{ctMarker, testKeyName, testKeyPassword, "5m0", "16777217"})
		})
	}
}

func TestClientOptionBoundariesAccepted(t *testing.T) {
	tests := []struct {
		name string
		opts []enformion.Option
	}{
		{name: "no options"},
		{name: "nil option", opts: []enformion.Option{nil}},
		{name: "timeout of one nanosecond", opts: []enformion.Option{enformion.WithTimeout(time.Nanosecond)}},
		{name: "timeout of exactly 5m", opts: []enformion.Option{enformion.WithTimeout(5 * time.Minute)}},
		{name: "max bytes of 1", opts: []enformion.Option{enformion.WithMaxResponseBytes(1)}},
		{name: "max bytes of exactly 16 MiB", opts: []enformion.Option{enformion.WithMaxResponseBytes(16 << 20)}},
		{name: "HTTP client", opts: []enformion.Option{enformion.WithHTTPClient(&http.Client{})}},
		{name: "test URL", opts: []enformion.Option{enformion.WithTestBaseURL("https://127.0.0.1:8443")}},
		{name: "provider diagnostics", opts: []enformion.Option{enformion.WithProviderDiagnostics()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construction performs no I/O, so the default client is safe here.
			c, err := enformion.NewClient(testKeyName, testKeyPassword, tt.opts...)
			require.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

func TestClientValidationOrder(t *testing.T) {
	badTimeout := enformion.WithTimeout(0)
	badBytes := enformion.WithMaxResponseBytes(0)
	badURL := enformion.WithTestBaseURL("http://127.0.0.1")
	tests := []struct {
		name     string
		keyName  string
		password string
		opts     []enformion.Option
		field    string
	}{
		{name: "key name before password", keyName: "", password: "", field: "keyName"},
		{name: "password before options", keyName: testKeyName, password: "", opts: []enformion.Option{enformion.WithHTTPClient(nil)}, field: "keyPassword"},
		{name: "HTTP client before timeout", keyName: testKeyName, password: testKeyPassword, opts: []enformion.Option{badTimeout, enformion.WithHTTPClient(nil)}, field: "HTTPClient"},
		{name: "timeout before max bytes", keyName: testKeyName, password: testKeyPassword, opts: []enformion.Option{badBytes, badTimeout}, field: "Timeout"},
		{name: "max bytes before test URL", keyName: testKeyName, password: testKeyPassword, opts: []enformion.Option{badURL, badBytes}, field: "MaxResponseBytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := enformion.NewClient(tt.keyName, tt.password, tt.opts...)
			require.Error(t, err)
			assert.Equal(t, tt.field, validationError(t, err).Field)
		})
	}
}

func TestClientLastOptionWins(t *testing.T) {
	t.Run("max response bytes", func(t *testing.T) {
		small, large := enformion.WithMaxResponseBytes(10), enformion.WithMaxResponseBytes(1<<20)

		c := newTransportClient(t, testutil.StaticTransport(http.StatusOK, emptyEnvelope), large, small)
		_, err := c.EnrichContact(t.Context(), validRequest())
		require.ErrorIs(t, err, enformion.ErrResponseTooLarge)

		c = newTransportClient(t, testutil.StaticTransport(http.StatusOK, emptyEnvelope), small, large)
		res, err := c.EnrichContact(t.Context(), validRequest())
		require.NoError(t, err)
		assert.Nil(t, res.Person)
	})
	t.Run("timeout", func(t *testing.T) {
		tests := []struct {
			name string
			opts []enformion.Option
			want time.Duration
		}{
			{name: "shorter last", opts: []enformion.Option{enformion.WithTimeout(time.Hour), enformion.WithTimeout(2 * time.Second)}, want: 2 * time.Second},
			{name: "longer last", opts: []enformion.Option{enformion.WithTimeout(2 * time.Second), enformion.WithTimeout(5 * time.Second)}, want: 5 * time.Second},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					c := newTransportClient(t, testutil.RoundTripFunc(testutil.BlockUntilDone), tt.opts...)
					start := time.Now()
					_, err := c.EnrichContact(t.Context(), validRequest())
					require.ErrorIs(t, err, enformion.ErrTimeout)
					assert.Equal(t, tt.want, time.Since(start))
				})
			})
		}
	})
}

func TestClientLeavesNilCallerFieldsNil(t *testing.T) {
	ct := &testutil.CountingTransport{Next: testutil.StaticTransport(http.StatusOK, emptyEnvelope)}
	hc := &http.Client{Transport: ct}
	c := newClient(t, enformion.WithTestBaseURL(loopbackOrigin), enformion.WithHTTPClient(hc))
	_, err := c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	assert.Nil(t, hc.CheckRedirect)
	assert.Nil(t, hc.Jar)
	assert.Zero(t, hc.Timeout)
	assert.Same(t, ct, hc.Transport)
	assert.Equal(t, 1, ct.Attempts())
}

func TestClientNotInitialized(t *testing.T) {
	var (
		zero      enformion.Client
		nilClient *enformion.Client
		nilCtx    context.Context
	)
	tests := []struct {
		name   string
		client *enformion.Client
		ctx    context.Context //nolint:containedctx // each case runs with its own context
		req    enformion.EnrichContactRequest
		search enformion.PersonSearchRequest
	}{
		{name: "zero client", client: &zero, ctx: context.Background(), req: validRequest(), search: validSearch()},
		{name: "nil client", client: nilClient, ctx: context.Background(), req: validRequest(), search: validSearch()},
		{name: "nil client before an invalid request", client: nilClient, ctx: context.Background()},
		{name: "zero client before a nil context", client: &zero, ctx: nilCtx, req: validRequest(), search: validSearch()},
	}
	want := enformion.ValidationError{Err: enformion.ErrInvalidConfig, Field: "Client", Reason: "not_initialized"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.client.EnrichContact(tt.ctx, tt.req)
			assert.Nil(t, res)
			require.ErrorIs(t, err, enformion.ErrInvalidConfig)
			assert.Equal(t, want, *validationError(t, err))

			page, err := tt.client.SearchPersons(tt.ctx, tt.search)
			assert.Nil(t, page)
			require.ErrorIs(t, err, enformion.ErrInvalidConfig)
			assert.Equal(t, want, *validationError(t, err))
		})
	}
}
