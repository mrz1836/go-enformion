package enformion

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// stManyKeys returns a JSON object with n distinct keys, prefix0 to
// prefix(n-1), each holding its index, plus extra members appended verbatim.
// prefix is spelled as JSON, so it may hold escapes.
func stManyKeys(prefix string, n int, extra string) string {
	var b strings.Builder
	b.WriteByte('{')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"` + prefix + strconv.Itoa(i) + `":` + strconv.Itoa(i))
	}
	b.WriteString(extra)
	b.WriteByte('}')
	return b.String()
}

func TestStrictCheckStrict(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		kind  FailureKind
		cause error
	}{
		{name: "valid object", data: `{"a":1,"b":[{"c":true}],"d":null}`},
		{name: "empty object", data: `{}`},
		{name: "surrounding whitespace", data: " \n\t{\"a\":1} \r\n"},
		{name: "invalid UTF-8 in a string", data: "{\"a\":\"\xff\"}", kind: FailureMalformedResponse, cause: errInvalidUTF8},
		{name: "truncated multibyte sequence", data: "{\"a\":\"\xe2\x82\"}", kind: FailureMalformedResponse, cause: errInvalidUTF8},
		{name: "UTF-8 byte order mark", data: "\xef\xbb\xbf{}", kind: FailureMalformedResponse, cause: errInvalidJSON},
		{name: "empty body", data: "", kind: FailureMalformedResponse, cause: errInvalidJSON},
		{name: "missing value", data: `{"a":}`, kind: FailureMalformedResponse, cause: errInvalidJSON},
		{name: "unterminated object", data: `{"a":1`, kind: FailureMalformedResponse, cause: errInvalidJSON},
		{name: "trailing comma", data: `{"a":[1,]}`, kind: FailureMalformedResponse, cause: errInvalidJSON},
		{name: "second object", data: `{} {}`, kind: FailureMalformedResponse, cause: errTrailingData},
		{name: "trailing garbage", data: `{}x`, kind: FailureMalformedResponse, cause: errTrailingData},
		{name: "top-level array", data: `[{"a":1}]`, kind: FailureUnexpectedResponse, cause: errNotObject},
		{name: "top-level array with a duplicate", data: `[{"a":1,"a":2}]`, kind: FailureUnexpectedResponse, cause: errNotObject},
		{name: "top-level string", data: `"x"`, kind: FailureUnexpectedResponse, cause: errNotObject},
		{name: "top-level number", data: `42`, kind: FailureUnexpectedResponse, cause: errNotObject},
		{name: "top-level null", data: jsonNull, kind: FailureUnexpectedResponse, cause: errNotObject},
		{name: "top-level true", data: ` true `, kind: FailureUnexpectedResponse, cause: errNotObject},
		{name: "duplicate at top level", data: `{"a":1,"a":2}`, kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "duplicate in a nested object", data: `{"a":{"b":1,"c":2,"b":3}}`, kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "duplicate in an object inside an array", data: `{"a":[{"x":1},{"y":1,"y":2}]}`, kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "duplicate spelled with an escape", data: testutil.Esc(`{"a":1,"~u0061":2}`), kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "duplicate in an ignored subtree", data: `{"isError":false,"searchCriteria":[{"q":1,"q":1}]}`, kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "same key in sibling objects", data: `{"a":{"k":1},"b":{"k":2},"c":[{"k":3},{"k":4}]}`},
		{name: "same key at different depths", data: `{"k":{"k":{"k":1}}}`},
		{name: "keys differing only by case", data: `{"a":1,"A":2,"person":null,"Person":null}`},
		{name: "a key equal to a string value", data: `{"a":"a","b":"a"}`},
		{name: "17 distinct keys", data: stManyKeys("k", 17, "")},
		{name: "40 distinct keys", data: stManyKeys("k", 40, "")},
		{name: "40 keys with a duplicate", data: stManyKeys("k", 40, `,"k5":0`), kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "17 keys with an escaped duplicate", data: stManyKeys("k", 17, testutil.Esc(`,"~u006b16":0`)), kind: FailureMalformedResponse, cause: errDuplicateKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, cause := checkStrict([]byte(tt.data))
			assert.Equal(t, tt.kind, kind)
			if tt.cause == nil {
				require.NoError(t, cause)
				return
			}
			require.ErrorIs(t, cause, tt.cause)
		})
	}
}

func TestStrictObjectMembers(t *testing.T) {
	data := []byte(" {\n \"a\" : 1 , \"b\":\"x,y\" ,\"c\":{\"d\":[1,2]}, \"e\":[ ], \"f\":null, \"g\":true, " +
		testutil.Esc(`"~u0068":-1.5e3`) + " } ")
	got := stMembers(data)
	require.Len(t, got, 7)
	want := []struct{ key, value string }{
		{"a", `1`},
		{"b", `"x,y"`},
		{"c", `{"d":[1,2]}`},
		{"e", `[ ]`},
		{"f", jsonNull},
		{"g", `true`},
		{"h", `-1.5e3`},
	}
	for i, w := range want {
		assert.Equal(t, w.key, got[i].key)
		assert.Equal(t, w.value, got[i].value)
	}
	assert.Empty(t, stMembers([]byte(`{}`)))
	assert.Empty(t, stMembers([]byte(` { } `)))
	assert.Nil(t, stMembers([]byte(`[1]`)))
	assert.Nil(t, stMembers(nil))

	// Breaking out of the loop stops the walk.
	n := 0
	for range members(data) {
		n++
		break
	}
	assert.Equal(t, 1, n)
}

func TestStrictArrayElements(t *testing.T) {
	data := []byte(` [ 1 , "a]" , {"b":[2]} , [3,[4]] , null , true , -0.5 ] `)
	assert.Equal(t, []string{`1`, `"a]"`, `{"b":[2]}`, `[3,[4]]`, jsonNull, `true`, `-0.5`}, stElements(data))
	assert.Empty(t, stElements([]byte(`[]`)))
	assert.Empty(t, stElements([]byte(`[ ]`)))
	assert.Nil(t, stElements([]byte(`{}`)))
	assert.Nil(t, stElements(nil))

	// Breaking out of the loop stops the walk.
	n := 0
	for range elements(data) {
		n++
		break
	}
	assert.Equal(t, 1, n)
}

// stMember is one member as members yields it, with the key decoded.
type stMember struct{ key, value string }

// stMembers collects what members yields.
func stMembers(data []byte) []stMember {
	var out []stMember
	for k, v := range members(data) {
		out = append(out, stMember{key: decodeKey(k), value: string(v)})
	}
	return out
}

// stElements collects what elements yields.
func stElements(data []byte) []string {
	var out []string
	for el := range elements(data) {
		out = append(out, string(el))
	}
	return out
}

func TestStrictValueEnd(t *testing.T) {
	tests := []struct {
		data string
		end  int
	}{
		{`"abc",`, 5},
		{testutil.Esc(`"a~"b",`), 6},
		{testutil.Esc(`"a~~",`), 5},
		{`{"a":"}"},`, 9},
		{`[1,[2,"]"],3] `, 13},
		{`123,`, 3},
		{`-1.5e+3}`, 7},
		{`true]`, 4},
		{`null `, 4},
		{`false:`, 5},
		{"1\n", 1},
		{`{"a":`, 5},
		{`"abc`, 4},
		{``, 0},
	}
	for _, tt := range tests {
		t.Run(tt.data, func(t *testing.T) {
			assert.Equal(t, tt.end, valueEnd([]byte(tt.data), 0))
		})
	}
	assert.Equal(t, 3, valueEnd([]byte("abc"), 5), "an index past the end returns the length")
}

func TestStrictStringEnd(t *testing.T) {
	tests := []struct {
		data string
		end  int
	}{
		{`""`, 2},
		{`"a"x`, 3},
		{testutil.Esc(`"~""`), 4},
		{testutil.Esc(`"~~"`), 4},
		{testutil.Esc(`"~~~""`), 6},
		{testutil.Esc(`"~u0022"`), 8},
		{`"abc`, 4},
		{testutil.Esc(`"~`), 2},
	}
	for _, tt := range tests {
		t.Run(tt.data, func(t *testing.T) {
			assert.Equal(t, tt.end, stringEnd([]byte(tt.data), 0))
		})
	}
}

func TestStrictSkipSpace(t *testing.T) {
	assert.Equal(t, 4, skipSpace([]byte(" \t\n\rx"), 0))
	assert.Equal(t, 0, skipSpace([]byte("x "), 0))
	assert.Equal(t, 2, skipSpace([]byte("  "), 0))
	assert.Equal(t, 1, skipSpace([]byte("x\u00a0"), 1), "only JSON whitespace is skipped")
	assert.Equal(t, 0, skipSpace([]byte("\vx"), 0), "only JSON whitespace is skipped")
	assert.Equal(t, 5, skipSpace([]byte("abc"), 5))
}

func TestStrictHasData(t *testing.T) {
	tests := []struct {
		data string
		want bool
	}{
		{`""`, false},
		{`"x"`, true},
		{`" "`, true},
		{testutil.Esc(`"~""`), true},
		{testutil.Esc(`"~u0000"`), true},
		{`0`, false},
		{`-0`, false},
		{`0.0`, false},
		{`0e5`, false},
		{`-0.000E-10`, false},
		{`1e-3`, true},
		{`0.1`, true},
		{`-7`, true},
		{`true`, true},
		{`false`, false},
		{jsonNull, false},
		{`{}`, false},
		{`[]`, false},
		{`{"k":""}`, false},
		{`{"key":0}`, false},
		{`{"key":false,"other":null}`, false},
		{`{"a":{"b":[[],[{}],{"c":""}]}}`, false},
		{`{"a":{"b":[[],[{"d":"x"}]]}}`, true},
		{`[[[0,0.0,false,null,""]]]`, false},
		{`[[[0,0.0,false,null,"",1]]]`, true},
		{`{"true":false}`, false},
		{`{"t":"","u":[true]}`, true},
		{testutil.Esc(`{"a~":b":""}`), false},
		{testutil.Esc(`{"k":"~"x~""}`), true},
		{` {"k" : "" , "j" : 0 } `, false},
	}
	for _, tt := range tests {
		t.Run(tt.data, func(t *testing.T) {
			assert.Equal(t, tt.want, hasData([]byte(tt.data)))
		})
	}
}

func TestStrictNonZeroNumber(t *testing.T) {
	tests := []struct {
		lit  string
		want bool
	}{
		{"0", false},
		{"-0", false},
		{"0.0", false},
		{"0e5", false},
		{"0E+9", false},
		{"0.000", false},
		{"1", true},
		{"-1", true},
		{"0.001", true},
		{"10", true},
		{"1e-3", true},
		{"0e1", false},
		{"9", true},
	}
	for _, tt := range tests {
		t.Run(tt.lit, func(t *testing.T) {
			assert.Equal(t, tt.want, nonZeroNumber([]byte(tt.lit)))
		})
	}
}

func TestStrictDecodeKey(t *testing.T) {
	tests := []struct {
		name string
		lit  string
		want string
	}{
		{name: "plain", lit: `"person"`, want: personKey},
		{name: "empty", lit: `""`, want: ""},
		{name: "unicode escape", lit: testutil.Esc(`"~u0070erson"`), want: personKey},
		{name: "control escapes", lit: testutil.Esc(`"a~nb~tc~/d~~e~"f"`), want: "a\nb\tc/d\\e\"f"},
		{name: "surrogate pair", lit: testutil.Esc(`"~ud83d~ude00"`), want: "\U0001F600"},
		{name: "lone surrogate", lit: testutil.Esc(`"~ud800"`), want: "\ufffd"},
		{name: "raw multibyte", lit: `"é"`, want: "é"},
		{name: "too short", lit: `"`, want: ""},
		{name: "nothing", lit: ``, want: ""},
		{name: "invalid escape decodes to nothing", lit: testutil.Esc(`"~x"`), want: ""},
		{name: "invalid hex digit decodes to nothing", lit: testutil.Esc(`"~u12zz"`), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, decodeKey([]byte(tt.lit)))
		})
	}
}

func TestStrictFoldIndex(t *testing.T) {
	names := []string{personKey, "isError", "key", "ks"}
	tests := []struct {
		key  string
		want int
	}{
		{personKey, 0},
		{"PERSON", 0},
		{"Person", 0},
		{"pErSoN", 0},
		{"iserror", 1},
		{"ISERROR", 1},
		{"key", 2},
		{"KEY", 2},
		{"\u212aey", -1}, // the Kelvin sign folds to k in Unicode, never here
		{"k\u017f", -1},  // the long s folds to s in Unicode, never here
		{"\u212a\u017f", -1},
		{"persons", -1},
		{"perso", -1},
		{"", -1},
		{"pers0n", -1},
		{"person ", -1},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, foldIndex([]byte(tt.key), names))
		})
	}
	assert.Equal(t, -1, foldIndex([]byte("a"), nil))
}

func TestStrictASCIIEqualFold(t *testing.T) {
	assert.True(t, asciiEqualFold([]byte("ab"), "AB"))
	assert.True(t, asciiEqualFold([]byte("a-Z_9"), "A-z_9"))
	assert.True(t, asciiEqualFold([]byte(""), ""))
	assert.False(t, asciiEqualFold([]byte("ab"), "ac"))
	assert.False(t, asciiEqualFold([]byte("a@"), "a`"), "only letters fold")
	assert.False(t, asciiEqualFold([]byte("["), "{"), "only letters fold")
	assert.False(t, asciiEqualFold([]byte("é"), "é"), "non-ASCII bytes never match")
	assert.Equal(t, byte('a'), lowerASCII('A'))
	assert.Equal(t, byte('z'), lowerASCII('Z'))
	assert.Equal(t, byte('a'), lowerASCII('a'))
	assert.Equal(t, byte('@'), lowerASCII('@'))
	assert.Equal(t, byte('['), lowerASCII('['))
}

func TestStrictIsNullAndIsKey(t *testing.T) {
	assert.True(t, isNull(nil))
	assert.True(t, isNull([]byte{}))
	assert.True(t, isNull([]byte(jsonNull)))
	assert.False(t, isNull([]byte("nul")))
	assert.False(t, isNull([]byte(`"null"`)))
	assert.False(t, isNull([]byte("0")))

	data := []byte(`{"a" : "b", "c":"d"}`)
	assert.True(t, isKey(data, 4), `"a" is a key`)
	assert.False(t, isKey(data, 10), `"b" is a value`)
	assert.False(t, isKey(data, len(data)))
}
