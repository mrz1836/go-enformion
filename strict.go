package enformion

import (
	"bytes"
	"encoding/json"
	"iter"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// linearKeys is the largest object whose keys are compared pairwise; larger
// objects are sorted once, so that a check stays O(n log n).
const linearKeys = 16

// checkStrict validates a whole response body before anything is decoded. It
// returns the failure kind and a fixed cause, or an empty kind when data is one
// JSON object in valid UTF-8, with nothing after it and no object anywhere that
// repeats a key once escapes are decoded.
//
// encoding/json alone would accept invalid UTF-8 (replacing it), and would let
// a repeated key silently override an earlier one.
func checkStrict(data []byte) (FailureKind, error) {
	if !utf8.Valid(data) {
		return FailureMalformedResponse, errInvalidUTF8
	}
	if !json.Valid(data) {
		return FailureMalformedResponse, invalidJSONCause(data)
	}
	if i := skipSpace(data, 0); i >= len(data) || data[i] != '{' {
		return FailureUnexpectedResponse, errNotObject
	}
	if duplicateKey(data) {
		return FailureMalformedResponse, errDuplicateKey
	}
	return "", nil
}

// invalidJSONCause tells data after a complete value apart from invalid syntax.
// Neither cause quotes the body.
func invalidJSONCause(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var v json.RawMessage
	if err := dec.Decode(&v); err != nil {
		return errInvalidJSON
	}
	return errTrailingData
}

// duplicateKey reports whether any object in data, at any depth, has two keys
// that are equal once unescaped, so "a" and "\u0061" collide. data must be
// valid JSON; for anything else the result is unspecified, but it never panics.
//
// Each open key costs 12 bytes: a key without escapes is kept as a span of
// data, and one with escapes is decoded once into a side buffer, so even a body
// made only of keys costs a small multiple of its size.
func duplicateKey(data []byte) bool {
	ks := keySet{data: data}
	var starts []int // per open container: where its keys begin, or -1 for an array
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '{':
			starts = append(starts, len(ks.spans))
		case '[':
			starts = append(starts, -1)
		case '}', ']':
			n := len(starts) - 1
			if n < 0 {
				return false
			}
			if start := starts[n]; start >= 0 {
				if ks.repeated(start) {
					return true
				}
				ks.spans = ks.spans[:start]
			}
			starts = starts[:n]
		case '"':
			end := stringEnd(data, i)
			if isKey(data, end) {
				ks.add(data[i:end], i)
			}
			i = end - 1
		}
	}
	return false
}

// keySet holds the keys of every open object as spans: of data for a key
// without escapes, or of decoded for one with escapes.
type keySet struct {
	data    []byte
	decoded []byte
	spans   []keySpan
}

// keySpan locates one decoded key.
type keySpan struct {
	start, end int32
	escaped    bool
}

// add records the key whose literal, quotes included, starts at data[at].
func (ks *keySet) add(lit []byte, at int) {
	if unescaped(lit) {
		ks.spans = append(ks.spans, keySpan{start: int32(at + 1), end: int32(at + len(lit) - 1)}) //nolint:gosec // bodies are at most 16 MiB
		return
	}
	start := len(ks.decoded)
	ks.decoded = appendUnquoted(ks.decoded, lit)
	ks.spans = append(ks.spans, keySpan{start: int32(start), end: int32(len(ks.decoded)), escaped: true}) //nolint:gosec // bodies are at most 16 MiB
}

// bytes returns the decoded content of the key at span s.
func (ks *keySet) bytes(s keySpan) []byte {
	if s.escaped {
		return ks.decoded[s.start:s.end]
	}
	return ks.data[s.start:s.end]
}

// repeated reports whether the keys from spans[start:] hold the same key
// twice. It may reorder them.
func (ks *keySet) repeated(start int) bool {
	keys := ks.spans[start:]
	if len(keys) <= linearKeys {
		for i := 1; i < len(keys); i++ {
			for _, k := range keys[:i] {
				if bytes.Equal(ks.bytes(k), ks.bytes(keys[i])) {
					return true
				}
			}
		}
		return false
	}
	slices.SortFunc(keys, func(a, b keySpan) int { return bytes.Compare(ks.bytes(a), ks.bytes(b)) })
	for i := 1; i < len(keys); i++ {
		if bytes.Equal(ks.bytes(keys[i]), ks.bytes(keys[i-1])) {
			return true
		}
	}
	return false
}

// keyBytes returns the decoded content of the JSON string literal lit, quotes
// included. A key without escapes is returned in place, without allocating.
func keyBytes(lit []byte) []byte {
	if unescaped(lit) {
		return lit[1 : len(lit)-1 : len(lit)-1]
	}
	// Decoding never lengthens the content, so one allocation suffices.
	return appendUnquoted(make([]byte, 0, len(lit)), lit)
}

// unescaped reports whether the JSON string literal lit, quotes included, has
// no escapes, so that its content is lit without the quotes.
func unescaped(lit []byte) bool { return len(lit) >= 2 && bytes.IndexByte(lit, '\\') < 0 }

// members yields the members of the JSON object that data holds, in order:
// each key as its raw string literal, quotes included, and its raw value. Both
// alias data. data must be a valid JSON object, optionally surrounded by
// whitespace; for anything else it yields what it can and never panics. It
// allocates nothing, so a huge object costs no memory to walk.
func members(data []byte) iter.Seq2[[]byte, []byte] {
	return func(yield func(key, value []byte) bool) {
		i := skipSpace(data, 0)
		if i >= len(data) || data[i] != '{' {
			return
		}
		for i++; ; i++ {
			i = skipSpace(data, i)
			if i >= len(data) || data[i] != '"' {
				return
			}
			kend := stringEnd(data, i)
			key := data[i:kend]
			i = skipSpace(data, kend)
			if i >= len(data) || data[i] != ':' {
				return
			}
			i = skipSpace(data, i+1)
			vend := valueEnd(data, i)
			if !yield(key, data[i:vend]) {
				return
			}
			i = skipSpace(data, vend)
			if i >= len(data) || data[i] != ',' {
				return
			}
		}
	}
}

// elements yields the raw elements of the JSON array that data holds, in
// order, aliasing data. data must be a valid JSON array; for anything else it
// yields what it can and never panics. It allocates nothing.
func elements(data []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		i := skipSpace(data, 0)
		if i >= len(data) || data[i] != '[' {
			return
		}
		for i++; ; i++ {
			i = skipSpace(data, i)
			if i >= len(data) || data[i] == ']' {
				return
			}
			end := valueEnd(data, i)
			if !yield(data[i:end]) {
				return
			}
			i = skipSpace(data, end)
			if i >= len(data) || data[i] != ',' {
				return
			}
		}
	}
}

// hasData reports whether the JSON value data holds any data at any depth: a
// non-empty string value (keys do not count), a non-zero number, or true.
func hasData(data []byte) bool {
	for i := 0; i < len(data); i++ {
		switch c := data[i]; {
		case c == '"':
			end := stringEnd(data, i)
			if end-i > 2 && !isKey(data, end) {
				return true
			}
			i = end - 1
		case c == 't':
			// Outside strings, 't' only starts the literal true.
			return true
		case c == '-' || (c >= '0' && c <= '9'):
			end := valueEnd(data, i)
			if nonZeroNumber(data[i:end]) {
				return true
			}
			i = end - 1
		}
	}
	return false
}

// nonZeroNumber reports whether the JSON number lit has a non-zero digit before
// any exponent, so that 0, -0, 0.0 and 0e5 are all zero.
func nonZeroNumber(lit []byte) bool {
	for _, c := range lit {
		switch {
		case c == 'e' || c == 'E':
			return false
		case c >= '1' && c <= '9':
			return true
		}
	}
	return false
}

// isNull reports whether the raw JSON value is null or absent.
func isNull(raw []byte) bool { return len(raw) == 0 || string(raw) == "null" }

// decodeKey returns the content of the JSON string literal lit, quotes
// included, decoded exactly as encoding/json decodes it.
func decodeKey(lit []byte) string { return string(keyBytes(lit)) }

// appendUnquoted appends the content of the JSON string literal lit, quotes
// included, to dst, decoded as encoding/json decodes it: escapes are resolved,
// a valid surrogate pair becomes one rune, and any other surrogate escape
// becomes U+FFFD. lit must come from a valid document, so its bytes are valid
// UTF-8 and copied as they are; at the first malformed escape, which a valid
// document never has, decoding stops.
func appendUnquoted(dst, lit []byte) []byte {
	if len(lit) < 2 {
		return dst
	}
	content := lit[1 : len(lit)-1]
	for i := 0; i < len(content); {
		c := content[i]
		if c != '\\' {
			dst = append(dst, c)
			i++
			continue
		}
		if i+1 >= len(content) {
			return dst
		}
		switch e := content[i+1]; e {
		case '"', '\\', '/':
			dst = append(dst, e)
		case 'b':
			dst = append(dst, '\b')
		case 'f':
			dst = append(dst, '\f')
		case 'n':
			dst = append(dst, '\n')
		case 'r':
			dst = append(dst, '\r')
		case 't':
			dst = append(dst, '\t')
		case 'u':
			r := hexEscape(content, i)
			if r < 0 {
				return dst
			}
			i += 6
			if utf16.IsSurrogate(r) {
				if low := hexEscape(content, i); r < 0xdc00 && isLowSurrogate(low) {
					r = utf16.DecodeRune(r, low)
					i += 6
				} else {
					r = utf8.RuneError
				}
			}
			dst = utf8.AppendRune(dst, r)
			continue
		default:
			return dst
		}
		i += 2
	}
	return dst
}

// foldIndex returns the index of the name in names that the decoded key
// equals, ignoring ASCII case, or -1. The names are ASCII, so a key with any
// other byte never matches.
func foldIndex(key []byte, names []string) int {
	for i, name := range names {
		if len(key) == len(name) && asciiEqualFold(key, name) {
			return i
		}
	}
	return -1
}

func asciiEqualFold(a []byte, b string) bool {
	for i := range len(a) {
		x, y := a[i], b[i]
		if x >= utf8.RuneSelf || y >= utf8.RuneSelf {
			return false
		}
		if x != y && lowerASCII(x) != lowerASCII(y) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// stringEnd returns the index just past the JSON string whose opening quote is
// at data[i], or len(data) when it is not terminated.
func stringEnd(data []byte, i int) int {
	for j := i + 1; j < len(data); j++ {
		switch data[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(data)
}

// valueEnd returns the index just past the JSON value that starts at data[i].
// data must be valid JSON from i on; otherwise the result is at most len(data).
func valueEnd(data []byte, i int) int {
	if i >= len(data) {
		return len(data)
	}
	switch data[i] {
	case '"':
		return stringEnd(data, i)
	case '{', '[':
		depth := 0
		for j := i; j < len(data); j++ {
			switch data[j] {
			case '"':
				j = stringEnd(data, j) - 1
			case '{', '[':
				depth++
			case '}', ']':
				if depth--; depth == 0 {
					return j + 1
				}
			}
		}
		return len(data)
	}
	j := i
	for j < len(data) && !strings.ContainsRune(",}]: \t\n\r", rune(data[j])) {
		j++
	}
	return j
}

// isKey reports whether the string ending just before data[end] is an object
// key: in valid JSON, exactly the strings followed by a colon.
func isKey(data []byte, end int) bool {
	j := skipSpace(data, end)
	return j < len(data) && data[j] == ':'
}

// skipSpace returns the index of the first non-whitespace byte at or after i.
func skipSpace(data []byte, i int) int {
	for i < len(data) {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// replaceLoneSurrogates returns the JSON text data with every \u escape of a
// UTF-16 surrogate that is not half of a valid pair replaced by \ufffd, the
// replacement character encoding/json decodes it to. Such escapes are valid
// JSON, but encoding/json/v2 refuses to marshal them. data is returned as it
// is, without allocating, when it has none.
func replaceLoneSurrogates(data []byte) []byte {
	var out []byte
	last := 0
	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}
		end := stringEnd(data, i)
		for j := i + 1; j < end-1; j++ {
			if data[j] != '\\' {
				continue
			}
			r := hexEscape(data, j)
			switch {
			case r < 0:
				j++ // a two-byte escape such as \n or \"
			case !utf16.IsSurrogate(r):
				j += 5
			case r < 0xdc00 && isLowSurrogate(hexEscape(data, j+6)):
				j += 11 // a valid pair
			default:
				out = append(append(out, data[last:j]...), `\ufffd`...)
				last = j + 6
				j += 5
			}
		}
		i = end - 1
	}
	if out == nil {
		return data
	}
	return append(out, data[last:]...)
}

// hexEscape returns the value of the \uXXXX escape at data[i], or -1 when
// there is none.
func hexEscape(data []byte, i int) rune {
	if i+6 > len(data) || data[i] != '\\' || data[i+1] != 'u' {
		return -1
	}
	var r rune
	for _, c := range data[i+2 : i+6] {
		switch {
		case c >= '0' && c <= '9':
			c -= '0'
		case c >= 'a' && c <= 'f':
			c -= 'a' - 10
		case c >= 'A' && c <= 'F':
			c -= 'A' - 10
		default:
			return -1
		}
		r = r<<4 | rune(c)
	}
	return r
}

func isLowSurrogate(r rune) bool { return r >= 0xdc00 && r <= 0xdfff }
