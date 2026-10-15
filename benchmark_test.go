package enformion_test

import (
	"net/http"
	"testing"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// These benchmarks measure local CPU and allocations through an in-memory
// transport. They are not evidence of network or provider performance.

func BenchmarkEnrichContactMatch(b *testing.B) {
	bmEnrich(b, string(testutil.Fixture(b, "provider/match.json")), true)
}

func BenchmarkEnrichContactNoPerson(b *testing.B) {
	bmEnrich(b, string(testutil.Fixture(b, "synthetic/no-person.json")), false)
}

// bmEnrich runs EnrichContact against an in-memory 200 response with body.
func bmEnrich(b *testing.B, body string, wantPerson bool) {
	b.Helper()
	client := newTransportClient(b, testutil.StaticTransport(http.StatusOK, body))
	req := validRequest()
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		res, err := client.EnrichContact(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
		if (res.Person != nil) != wantPerson {
			b.Fatalf("person present: %t, want %t", res.Person != nil, wantPerson)
		}
	}
}

func BenchmarkNewClient(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := enformion.NewClient(testKeyName, testKeyPassword); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEnrichContactValidationError(b *testing.B) {
	ct := &testutil.CountingTransport{Next: testutil.StaticTransport(http.StatusOK, `{}`)}
	client := newTransportClient(b, ct)
	req := enformion.EnrichContactRequest{Email: "avery.example@example.com"}
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := client.EnrichContact(ctx, req); err == nil {
			b.Fatal("want a validation error")
		}
	}
	if ct.Attempts() != 0 {
		b.Fatalf("%d attempts, want 0", ct.Attempts())
	}
}
