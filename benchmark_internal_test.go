package enformion

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// These benchmarks measure local CPU and allocations only.

// bmNearMaxBody returns a valid response of just under the default 1 MiB limit:
// the match fixture with its empty searchCriteria array, which is never
// decoded, replaced by a large one.
func bmNearMaxBody(b *testing.B) []byte {
	b.Helper()
	match := testutil.Fixture(b, "provider/match.json")
	const empty = `"searchCriteria": []`
	if bytes.Count(match, []byte(empty)) != 1 {
		b.Fatal("the match fixture must hold one empty searchCriteria array")
	}
	const entry = `{"field":"FirstName","value":"Avery"}`
	var buf bytes.Buffer
	buf.WriteString(`"searchCriteria":[`)
	for n := 0; buf.Len()+len(entry)+len(match)+8 < defaultMaxResponseBytes; n++ {
		if n > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(entry)
	}
	buf.WriteByte(']')
	return bytes.Replace(match, []byte(empty), buf.Bytes(), 1)
}

func BenchmarkDecodeTypical(b *testing.B) {
	bmDecode(b, testutil.Fixture(b, "provider/match.json"))
}

func BenchmarkDecodeNearMax(b *testing.B) {
	body := bmNearMaxBody(b)
	if len(body) > defaultMaxResponseBytes {
		b.Fatalf("body is %d bytes, over the limit", len(body))
	}
	bmDecode(b, body)
}

// bmDecode decodes body as an HTTP 200 response, which must hold a person.
func bmDecode(b *testing.B, body []byte) {
	b.Helper()
	ex := &exchange{status: http.StatusOK, body: body}
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		res, err := decodeEnrichContact(ex, false)
		if err != nil || res.Person == nil {
			b.Fatalf("decode failed: %v", err)
		}
	}
}

func BenchmarkCheckStrictSmall(b *testing.B) {
	bmStrict(b, testutil.Fixture(b, "provider/match.json"))
}

func BenchmarkCheckStrict1MiB(b *testing.B) {
	bmStrict(b, bmNearMaxBody(b))
}

func bmStrict(b *testing.B, body []byte) {
	b.Helper()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := checkStrict(body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeRequest(b *testing.B) {
	req := rqFull()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := req.encode(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateRequest(b *testing.B) {
	req := rqFull()
	b.ReportAllocs()
	for b.Loop() {
		if err := req.validate(); err != nil {
			b.Fatal(err)
		}
	}
}
