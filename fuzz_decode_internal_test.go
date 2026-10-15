package enformion

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// fdWireNames are the only names a CallError Field may contain.
func fdWireNames() map[string]bool {
	names := map[string]bool{}
	for _, n := range []string{
		"person", "message", "identityScore", "requestId", "isError", "error",
		"personId", "name", "age", "partialDob", "addresses", "phones", "emails",
		"firstName", "middleName", "lastName",
		"street", "unit", "city", "state", "zip", "firstReportedDate", "lastReportedDate",
		"number", "type", "isConnected", "email", "isValidated", "isBusiness",
		"inputErrors", "warnings",
	} {
		names[n] = true
	}
	return names
}

// fdValidField reports whether field is empty or a dotted path of known wire
// names, each optionally followed by array indexes.
func fdValidField(field string, names map[string]bool, index *regexp.Regexp) bool {
	if field == "" {
		return true
	}
	for seg := range strings.SplitSeq(field, ".") {
		if !names[index.ReplaceAllString(seg, "")] {
			return false
		}
	}
	return true
}

// fdSeeds returns the fixtures and tricky bodies every fuzz target starts from.
func fdSeeds(f *testing.F) [][]byte {
	f.Helper()
	seeds := make([][]byte, 0, 32)
	for _, name := range []string{
		"provider/match.json", "provider/rejected-400.json",
		"synthetic/no-person.json", "synthetic/is-error.json",
		"synthetic/defaults-person.json", "synthetic/unknown-keys-person.json",
	} {
		seeds = append(seeds, testutil.Fixture(f, name))
	}
	for _, s := range []string{
		``, `{}`, `null`, `[]`, `{"isError":false}`, `{"isError":true}`,
		`{"isError":false,"person":{}}`,
		`{"isError":false,"person":{"age":31337}}`,
		`{"isError":false,"person":{"phones":[null,{"x":"y"}]}}`,
		`{"isError":false,"person":{"name":{"first":"A"},"age":"3"}}`,
		`{"isError":false,"Person":null,"person":null}`,
		`{"isError":false,"identityScore":1e2}`,
		`{"isError":false,"error":{"warnings":["<>&", null, {"a":[1]}]}}`,
		`{"isError":false,"message":"` + strings.Repeat("é", 600) + `"}`,
		`{"a":1,"a":2}`, `{"isError":false} x`, "\xef\xbb\xbf{}", "{\"a\":\"\xff\"}",
		testutil.Esc(`{"~u0069sError":false,"requestId":"r~u0069d"}`),
		testutil.Esc(`{"isError":false,"message":"~ud800","error":{"inputErrors":["~udc00~u2028"]}}`),
	} {
		seeds = append(seeds, []byte(s))
	}
	return seeds
}

func FuzzDecodeEnvelope(f *testing.F) {
	for _, seed := range fdSeeds(f) {
		f.Add(seed, uint16(200))
		f.Add(seed, uint16(429))
	}
	grammar := regexp.MustCompile(`^enformion: [a-z_]+( \(HTTP \d+\))?(: [A-Za-z0-9_.\[\]]+)?$`)
	index := regexp.MustCompile(`(\[\d+\])+$`)
	names := fdWireNames()

	f.Fuzz(func(t *testing.T, body []byte, alt uint16) {
		for _, status := range []int{200, 100 + int(alt)%900} {
			for _, diag := range []bool{false, true} {
				res, err := decodeEnrichContact(&exchange{status: status, body: body}, diag)
				if err != nil {
					assert.Nil(t, res)
					ce := asCallError(t, err)
					require.Regexp(t, grammar, err.Error())
					assert.True(t, fdValidField(ce.Field, names, index), "field %q", ce.Field)
					assert.Equal(t, status, ce.StatusCode)
					assert.True(t, ce.RequestID == "" || validID(ce.RequestID))
					if !diag {
						assert.Equal(t, ProviderDiagnostics{}, ce.Diagnostics())
					}
					continue
				}
				require.Equal(t, 200, status, "only a 200 succeeds")
				require.NotNil(t, res)
				assert.True(t, res.RequestID == "" || validID(res.RequestID))
				assert.LessOrEqual(t, len(res.Diagnostics.Message), maxDiagnosticMessageBytes)
				assert.True(t, utf8.ValidString(res.Diagnostics.Message))
				assert.LessOrEqual(t, len(res.Diagnostics.InputErrors), maxDiagnosticEntries)
				assert.LessOrEqual(t, len(res.Diagnostics.Warnings), maxDiagnosticEntries)
				data, merr := json.Marshal(res)
				require.NoError(t, merr)
				var back EnrichContactResult
				require.NoError(t, json.Unmarshal(data, &back))
				require.Equal(t, *res, back, "a decoded result round-trips through JSON")
			}
		}
	})
}

// fdStrictSeeds returns documents for the strict scanner: duplicate keys
// spelled every way, which the oracle must agree on, and malformed input that
// no helper may panic on.
func fdStrictSeeds() []string {
	return []string{
		`{}`, `[]`, `1`, `"a"`, jsonNull,
		`{"a":1,"b":2}`, `{"a":1,"a":2}`, `{"a":{"a":{"a":1}}}`, `{"a":"b","b":"a"}`, `{"a":1,"A":2}`,
		`[{"a":1},{"a":1}]`, `[{"a":1,"b":{"c":1,"c":1}}]`, `{"a":{"a":1},"b":[{"c":1,"c":2}]}`,
		`{"x":[1,2,{"y":[{"z":1},{"z":2}]}]}`, ` { "a" : 1 , "a" : 2 } `,
		testutil.Esc(`{"~u0061":1,"a":2}`), testutil.Esc(`{"~ud800":1,"~udc00":2}`),
		testutil.Esc(`{"a~"b":1,"a\"b":2}`), testutil.Esc(`{"~n":1,"~u000a":2}`), testutil.Esc(`{"~/":1,"/":2}`),
		`{"é":1,"é":2}`, testutil.Esc(`{"é":1,"e~u0301":2}`), testutil.Esc(`{"é":1,"~u00e9":2}`),
		stManyKeys("k", 17, `,"k3":0`), stManyKeys("k", 20, ""), stManyKeys("k", 20, `,"k19":1`), stManyKeys("k", 100, `,"k0":1`),

		"", " ", "{", "}", "[", "]", `"`, `"\`, `"\u12`, `{"a"`, `{"a":`, `{"a":1,`, `{"a" 1}`,
		`[1,`, `[,]`, `{,}`, `{"a":1]`, `[{"a":1}`, `]]]}}}`, `{{{{`, `"abc`, `tru`, `-`, `1e`,
		"\xff\xfe", `{"a":"\`, `{"\`, `:::`, `,,,`, `{"a":[}`, "\x00", `{"k":"v"}garbage`,
		`{"a":{"b":{"c":`, `[[[[[[`, `{"":""}`, `"\"\"\"`, `{"a":1,"a":`,
	}
}

// fdSweep runs every scanning helper over data, the index-based ones at every
// index, so that a panic on any input fails the fuzz target. Sweeping every
// index is quadratic, so only short inputs get it.
func fdSweep(data []byte) {
	_ = stMembers(data)
	_ = stElements(data)
	_ = replaceLoneSurrogates(data)
	_ = hasData(data)
	_ = decodeKey(data)
	_ = nonZeroNumber(data)
	_ = isNull(data)
	if len(data) > 4<<10 {
		return
	}
	for i := 0; i <= len(data)+1; i++ {
		_ = valueEnd(data, i)
		_ = skipSpace(data, i)
		_ = isKey(data, i)
		if i < len(data) {
			_ = stringEnd(data, i)
		}
	}
}

func FuzzStrictOracle(f *testing.F) {
	for _, seed := range fdSeeds(f) {
		f.Add(seed)
	}
	for _, s := range fdStrictSeeds() {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		kind, cause := checkStrict(data)
		assert.Equal(t, kind == "", cause == nil)
		fdSweep(data)
		dup := duplicateKey(data)

		if !utf8.Valid(data) || !json.Valid(data) {
			assert.NotEmpty(t, kind, "invalid input never passes the strict check")
			return
		}
		valid, oracleDup := testutil.TokenWalk(data)
		require.True(t, valid, "the oracle accepts every valid document")
		assert.Equal(t, oracleDup, dup, "duplicateKey agrees with the token-walk oracle")
		if kind == "" {
			assert.False(t, oracleDup)
			members := stMembers(data)
			var decoded map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &decoded))
			assert.Len(t, members, len(decoded), "members sees every member once")
		}
	})
}
