package enformion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

func TestEndpointTestOriginAccepts(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "https://127.0.0.1", want: "https://127.0.0.1"},
		{raw: "https://127.0.0.1:8443", want: "https://127.0.0.1:8443"},
		{raw: "https://127.0.0.1/", want: "https://127.0.0.1"},
		{raw: "https://127.0.0.1:1/", want: "https://127.0.0.1:1"},
		{raw: "https://127.0.0.1:65535", want: "https://127.0.0.1:65535"},
		{raw: "https://127.255.255.254", want: "https://127.255.255.254"},
		{raw: "https://127.1.2.3:443", want: "https://127.1.2.3:443"},
		{raw: "https://[::1]", want: "https://[::1]"},
		{raw: "https://[::1]:8443", want: "https://[::1]:8443"},
		{raw: "https://[::1]/", want: "https://[::1]"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := testOrigin(tt.raw)
			require.True(t, ok)
			assert.Equal(t, tt.want, got)

			c, err := NewClient("key", "password", WithTestBaseURL(tt.raw))
			require.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

func TestEndpointTestOriginRejects(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "localhost", raw: "https://localhost"},
		{name: "localhost with port", raw: "https://localhost:8443"},
		{name: "http", raw: "http://127.0.0.1"},
		{name: "no scheme", raw: "127.0.0.1:8443"},
		{name: "opaque", raw: "https:127.0.0.1"},
		{name: "userinfo", raw: "https://user@127.0.0.1"},
		{name: "userinfo with password", raw: "https://user:pw@127.0.0.1"}, //nolint:gosec // synthetic userinfo the validator must reject
		{name: "query", raw: "https://127.0.0.1?q=1"},
		{name: "empty query", raw: "https://127.0.0.1?"},
		{name: "fragment", raw: "https://127.0.0.1#frag"},
		{name: "path", raw: "https://127.0.0.1/Contact/Enrich"},
		{name: "dot path", raw: "https://127.0.0.1/."},
		{name: "escaped path", raw: "https://127.0.0.1/%2F"},
		{name: "backslash", raw: `https://127.0.0.1\`},
		{name: "port zero", raw: "https://127.0.0.1:0"},
		{name: "port too large", raw: "https://127.0.0.1:65536"},
		{name: "signed port", raw: "https://127.0.0.1:+1"},
		{name: "leading zero port", raw: "https://127.0.0.1:0443"},
		{name: "empty port", raw: "https://127.0.0.1:"},
		{name: "short IPv4", raw: "https://127.1"},
		{name: "octal IPv4", raw: "https://0177.0.0.1"},
		{name: "IPv4-mapped IPv6", raw: "https://[::ffff:127.0.0.1]"},
		{name: "long-form IPv6 loopback", raw: "https://[0:0:0:0:0:0:0:1]"},
		{name: "unbracketed IPv6", raw: "https://::1"},
		{name: "unclosed bracket", raw: "https://[::1"},
		{name: "IPv6 zone", raw: "https://[::1%25lo0]"},
		{name: "private address", raw: "https://10.0.0.1"},
		{name: "unspecified address", raw: "https://0.0.0.0"},
		{name: "provider host", raw: "https://devapi.enformion.com"},
		{name: "leading space", raw: " https://127.0.0.1"},
		{name: "trailing junk after bracket", raw: "https://[::1]x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := testOrigin(tt.raw)
			assert.False(t, ok)
			assert.Empty(t, got)

			c, err := NewClient("key", "password", WithTestBaseURL(tt.raw))
			assert.Nil(t, c)
			assertValidation(t, err, ErrInvalidConfig, "TestBaseURL", reasonInvalidURL)
			if tt.raw != "" {
				testutil.AssertRedacted(t, err.Error(), []string{tt.raw})
			}
		})
	}
}

func TestEndpointLoopbackHostPort(t *testing.T) {
	tests := []struct {
		hostport string
		ok       bool
	}{
		{hostport: "127.0.0.1", ok: true},
		{hostport: "127.0.0.1:443", ok: true},
		{hostport: "[::1]", ok: true},
		{hostport: "[::1]:443", ok: true},
		{hostport: "127.0.0.1:", ok: false},
		{hostport: "127.0.0.1:x", ok: false},
		{hostport: "[::1]443", ok: false},
		{hostport: "[::1]:", ok: false},
		{hostport: "[::2]", ok: false},
		{hostport: "128.0.0.1", ok: false},
		{hostport: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.hostport, func(t *testing.T) {
			assert.Equal(t, tt.ok, loopbackHostPort(tt.hostport))
		})
	}
}
