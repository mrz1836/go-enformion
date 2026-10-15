package enformion

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// bdBody returns a successful envelope whose person holds a personId and, under
// list, n comma-separated copies of entry.
func bdBody(list, entry string, n int) []byte {
	entries := strings.TrimSuffix(strings.Repeat(entry+",", n), ",")
	return []byte(dcPerson(`{"personId":"G-1","` + list + `":[` + entries + `]}`))
}

// TestBoundsListEntries proves the per-collection SDK bound: 1,000 entries
// decode, the 1,001st fails the call loudly (never truncated), and JSON null
// elements do not count.
func TestBoundsListEntries(t *testing.T) {
	for _, list := range []string{"addresses", "phones", "emails"} {
		t.Run(list, func(t *testing.T) {
			res, err := dcDecode(bdBody(list, `{}`, maxListEntries), false)
			require.NoError(t, err)
			require.NotNil(t, res.Person)

			_, err = dcDecode(bdBody(list, `{}`, maxListEntries+1), false)
			require.ErrorIs(t, err, ErrResponseTooLarge)
			ce := asCallError(t, err)
			assert.Equal(t, FailureResponseTooLarge, ce.Kind)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, "person."+list, ce.Field)

			_, err = dcDecode(bdBody(list, `{},null`, maxListEntries), false)
			require.NoError(t, err, "null elements are not entries")
		})
	}
}

// TestBoundsHostileBodiesStayCheap guards against memory amplification: a
// 1 MiB body of empty list entries or of diagnostics entries must cost a small
// multiple of its size, not hundreds of MiB.
func TestBoundsHostileBodiesStayCheap(t *testing.T) {
	testutil.SkipUnderRace(t)
	const size = 1 << 20
	fill := func(prefix, el, suffix string) []byte {
		n := (size - len(prefix) - len(suffix)) / (len(el) + 1)
		return []byte(prefix + strings.TrimSuffix(strings.Repeat(el+",", n), ",") + suffix)
	}
	// Each member, such as "k12345":12345, takes at most 14 bytes plus the
	// key prefix, so these keys fill about size bytes.
	keys := func(prefix string) []byte {
		return []byte(stManyKeys(prefix, size/(len(prefix)+14), `,"isError":false`))
	}
	tests := []struct {
		name  string
		body  []byte
		check func(t *testing.T, res *EnrichContactResult, err error)
	}{
		{
			name: "empty address entries",
			body: fill(`{"isError":false,"person":{"personId":"G-1","addresses":[`, `{}`, `]}}`),
			check: func(t *testing.T, _ *EnrichContactResult, err error) {
				require.ErrorIs(t, err, ErrResponseTooLarge)
			},
		},
		{
			name: "diagnostics entries",
			body: fill(`{"isError":false,"error":{"inputErrors":[`, `0`, `]}}`),
			check: func(t *testing.T, res *EnrichContactResult, err error) {
				require.NoError(t, err)
				assert.Len(t, res.Diagnostics.InputErrors, maxDiagnosticEntries)
			},
		},
		{
			name: "many distinct envelope keys",
			body: keys("k"),
			check: func(t *testing.T, res *EnrichContactResult, err error) {
				require.NoError(t, err)
				assert.Nil(t, res.Person)
			},
		},
		{
			name: "many distinct escaped keys",
			body: keys(testutil.Esc("~u006b")),
			check: func(t *testing.T, _ *EnrichContactResult, err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := dcDecode(tt.body, false)
			tt.check(t, res, err)
			grew := testutil.HeapGrowth(func() {
				_, _ = dcDecode(tt.body, false)
			})
			t.Logf("%d bytes allocated decoding a %d-byte body", grew, len(tt.body))
			assert.Less(t, grew, uint64(8*size), "decoding must cost a small multiple of the body")
		})
	}
}

// TestBoundsLoneSurrogates proves that diagnostics entries never keep an
// unpaired UTF-16 surrogate escape, which encoding/json/v2 refuses to marshal,
// while valid pairs and other escapes are kept as they are.
func TestBoundsLoneSurrogates(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{name: "no escapes", in: `{"k":"v"}`, want: `{"k":"v"}`},
		{name: "lone high", in: testutil.Esc(`{"k":"~ud800"}`), want: testutil.Esc(`{"k":"~ufffd"}`)},
		{name: "lone low", in: testutil.Esc(`["~udc00"]`), want: testutil.Esc(`["~ufffd"]`)},
		{name: "high then non-low", in: testutil.Esc(`"~ud800~u0041"`), want: testutil.Esc(`"~ufffd~u0041"`)},
		{name: "valid pair", in: testutil.Esc(`"~ud83d~ude00"`), want: testutil.Esc(`"~ud83d~ude00"`)},
		{name: "reversed pair", in: testutil.Esc(`"~ude00~ud83d"`), want: testutil.Esc(`"~ufffd~ufffd"`)},
		{name: "in a key", in: testutil.Esc(`{"~udbff":1}`), want: testutil.Esc(`{"~ufffd":1}`)},
		{name: "escaped backslash before u", in: testutil.Esc(`"~~ud800"`), want: testutil.Esc(`"~~ud800"`)},
		{name: "other escapes", in: testutil.Esc(`"~n~"~/~u00e9"`), want: testutil.Esc(`"~n~"~/~u00e9"`)},
		{name: "uppercase hex", in: testutil.Esc(`"~uD800"`), want: testutil.Esc(`"~ufffd"`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceLoneSurrogates([]byte(tt.in))
			assert.Equal(t, tt.want, string(got))
			assert.True(t, json.Valid(got))
		})
	}
	unchanged := []byte(`{"k":"v"}`)
	assert.Same(t, &unchanged[0], &replaceLoneSurrogates(unchanged)[0], "no copy when nothing changes")

	body := []byte(testutil.Esc(`{"isError":false,"error":{"warnings":[{"k":"~ud800"},"~udfff x"]}}`))
	res, err := dcDecode(body, false)
	require.NoError(t, err)
	require.Len(t, res.Diagnostics.Warnings, 2)
	for _, w := range res.Diagnostics.Warnings {
		assert.NotContains(t, strings.ToLower(string(w)), `\ud8`)
		assert.NotContains(t, strings.ToLower(string(w)), `\udf`)
	}
}

// TestBoundsPath covers the lazily rendered JSON paths.
func TestBoundsPath(t *testing.T) {
	assert.Equal(t, "person.name", at("person.name", -1).String())
	assert.Equal(t, "person.phones[3]", at("person.phones", 3).String())
	assert.Equal(t, "person.phones[3].number", at("person.phones", 3).member("number"))
	assert.Equal(t, "person.name.firstName", at("person.name", -1).member("firstName"))
}

// FuzzDecodeKey proves that the hand-written unescaper decodes every valid JSON
// string literal exactly as encoding/json does, under either JSON engine.
func FuzzDecodeKey(f *testing.F) {
	for _, s := range []string{
		`"plain"`, `""`, testutil.Esc(`"~u0061"`), testutil.Esc(`"~ud83d~ude00"`), testutil.Esc(`"~ud800"`),
		testutil.Esc(`"~ud800~u0041"`), testutil.Esc(`"~udc00~ud800"`), testutil.Esc(`"~ud800~ud800~udc00"`),
		testutil.Esc(`"~"~~~/~b~f~n~r~t"`), testutil.Esc(`"~u0000"`), testutil.Esc(`"é~u6f22~u5b57"`), testutil.Esc(`"~uD83D~uDE00"`),
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, lit []byte) {
		// The unescaper sees only exact string literals of bodies that passed
		// utf8.Valid and json.Valid.
		if len(lit) < 2 || lit[0] != '"' || stringEnd(lit, 0) != len(lit) || !utf8.Valid(lit) || !json.Valid(lit) {
			return
		}
		var want string
		require.NoError(t, json.Unmarshal(lit, &want))
		assert.Equal(t, want, decodeKey(lit))
		assert.Equal(t, want, string(keyBytes(lit)))
	})
}
