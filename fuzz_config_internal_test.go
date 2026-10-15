package enformion

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func FuzzTestBaseURL(f *testing.F) {
	for _, seed := range []string{
		"https://127.0.0.1", "https://127.0.0.1:8443", "https://[::1]:443", "https://127.0.0.1/",
		"https://localhost", "http://127.0.0.1", "https://user@127.0.0.1", "https://127.1",
		"https://0177.0.0.1", "https://[::ffff:127.0.0.1]", "https://127.0.0.1:0443", "https://127.0.0.1:",
		"https://127.0.0.1?q", "https://127.0.0.1#f", "https://127.0.0.1/p", "", "https://[::1",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		origin, ok := testOrigin(raw)
		if !ok {
			require.Empty(t, origin)
			return
		}
		require.True(t, strings.HasPrefix(origin, "https://127.") || strings.HasPrefix(origin, "https://[::1]"),
			"accepted origin %q is not loopback", origin)
		again, ok := testOrigin(origin)
		require.True(t, ok, "an accepted origin must be accepted again")
		require.Equal(t, origin, again)

		host := strings.TrimPrefix(origin, "https://")
		if h, _, found := strings.Cut(strings.TrimPrefix(host, "["), "]"); strings.HasPrefix(host, "[") && found {
			host = h
		} else if i := strings.LastIndexByte(host, ':'); i >= 0 {
			host = host[:i]
		}
		addr, err := netip.ParseAddr(host)
		require.NoError(t, err)
		require.True(t, addr.IsLoopback())
	})
}

func FuzzCredential(f *testing.F) {
	for _, seed := range []string{
		"", "k", "key name", " lead", "trail ", "a\rb", "a\nb", "a\x00b", "a\tb", "a\x7fb", "pässword",
		strings.Repeat("k", 1024), strings.Repeat("k", 1025), "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		err := checkCredential("keyName", s)

		want := s != "" && len(s) <= 1024 && s[0] != ' ' && s[len(s)-1] != ' '
		for i := 0; want && i < len(s); i++ {
			want = s[i] >= 0x20 && s[i] <= 0x7e
		}
		require.Equal(t, want, err == nil, "accepted = %v", err == nil)
		if err != nil {
			assert.Contains(t, []string{
				"enformion: invalid configuration: keyName: required",
				"enformion: invalid configuration: keyName: invalid_header_value",
			}, err.Error(), "the error must come from the fixed set, never quote the value")
			return
		}

		// An accepted credential survives a real HTTP/1 request unchanged.
		req, rerr := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://127.0.0.1/Contact/Enrich", http.NoBody)
		require.NoError(t, rerr)
		req.Header["Galaxy-Ap-Name"] = []string{s}
		var buf bytes.Buffer
		require.NoError(t, req.Write(&buf))
		back, rerr := http.ReadRequest(bufio.NewReader(&buf))
		require.NoError(t, rerr)
		require.Equal(t, []string{s}, back.Header["Galaxy-Ap-Name"])
	})
}
