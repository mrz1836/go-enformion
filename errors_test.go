package enformion_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// etTargets returns every sentinel and context error a CallError might match,
// in the order the matrix reports matches.
func etTargets() []error {
	return []error{
		enformion.ErrInvalidConfig, enformion.ErrInvalidRequest, enformion.ErrUnauthorized, enformion.ErrRateLimited,
		enformion.ErrRejected, enformion.ErrUpstream, enformion.ErrUnexpectedResponse, enformion.ErrMalformedResponse,
		enformion.ErrResponseTooLarge, enformion.ErrTransport, enformion.ErrTimeout,
		context.Canceled, context.DeadlineExceeded,
	}
}

// TestErrorsKindSentinelMatrix proves each kind matches exactly its sentinel,
// a timeout also context.DeadlineExceeded, and an unknown kind nothing, and
// that wrapping changes none of it.
func TestErrorsKindSentinelMatrix(t *testing.T) {
	sentinels := kindSentinels()
	kinds := append(slices.Collect(maps.Keys(sentinels)), "", "not_a_kind")
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			var want []error
			if s, ok := sentinels[kind]; ok {
				want = append(want, s)
			}
			if kind == enformion.FailureTimeout {
				want = append(want, context.DeadlineExceeded)
			}
			ce := &enformion.CallError{Kind: kind}
			var got []error
			for _, target := range etTargets() {
				if errors.Is(ce, target) {
					got = append(got, target)
				}
			}
			assert.Equal(t, want, got)

			wrapped := fmt.Errorf("enrich failed: %w", ce)
			for _, target := range etTargets() {
				assert.Equal(t, errors.Is(ce, target), errors.Is(wrapped, target), target.Error())
			}
		})
	}
}

func TestErrorsCallErrorText(t *testing.T) {
	tests := []struct {
		name string
		err  *enformion.CallError
		want string
	}{
		{name: "kind and status", err: &enformion.CallError{Kind: enformion.FailureRateLimited, StatusCode: 429}, want: "enformion: rate_limited (HTTP 429)"},
		{name: "kind only", err: &enformion.CallError{Kind: enformion.FailureTransport}, want: "enformion: transport"},
		{name: "kind, status and field", err: &enformion.CallError{Kind: enformion.FailureMalformedResponse, StatusCode: 200, Field: "person.age"}, want: "enformion: malformed_response (HTTP 200): person.age"},
		{name: "kind and field", err: &enformion.CallError{Kind: enformion.FailureUnexpectedResponse, Field: "isError"}, want: "enformion: unexpected_response: isError"},
		{name: "request ID is not printed", err: &enformion.CallError{Kind: enformion.FailureUpstream, StatusCode: 503, RequestID: "req-et-1"}, want: "enformion: upstream (HTTP 503)"},
		{name: "empty kind", err: &enformion.CallError{}, want: "enformion: call_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestErrorsGoString(t *testing.T) {
	ce := &enformion.CallError{Kind: enformion.FailureRateLimited, StatusCode: 429, RequestID: "req-1", Field: "x"}
	want := `&enformion.CallError{Kind:"rate_limited", StatusCode:429, RequestID:"req-1", Field:"x"}`
	assert.Equal(t, want, ce.GoString())
	assert.Equal(t, want, fmt.Sprintf("%#v", ce))
}

func TestErrorsNilReceivers(t *testing.T) {
	var ce *enformion.CallError
	assert.NotPanics(t, func() {
		assert.Equal(t, "enformion: <nil CallError>", ce.Error())
		assert.NoError(t, ce.Unwrap())
		assert.False(t, ce.Is(enformion.ErrTransport))
		assert.Equal(t, "(*enformion.CallError)(nil)", ce.GoString())
		assert.Equal(t, enformion.ProviderDiagnostics{}, ce.Diagnostics())
	})

	var ve *enformion.ValidationError
	assert.NotPanics(t, func() {
		assert.Equal(t, "enformion: <nil ValidationError>", ve.Error())
		assert.NoError(t, ve.Unwrap())
	})
}

func TestErrorsValidationErrorText(t *testing.T) {
	tests := []struct {
		name string
		err  *enformion.ValidationError
		want string
	}{
		{name: "request field", err: &enformion.ValidationError{Err: enformion.ErrInvalidRequest, Field: "DOB", Reason: "invalid_format"}, want: "enformion: invalid request: DOB: invalid_format"},
		{name: "config field", err: &enformion.ValidationError{Err: enformion.ErrInvalidConfig, Field: "Client", Reason: "not_initialized"}, want: "enformion: invalid configuration: Client: not_initialized"},
		{name: "no field", err: &enformion.ValidationError{Err: enformion.ErrInvalidConfig, Reason: "required"}, want: "enformion: invalid configuration: required"},
		{name: "sentinel only", err: &enformion.ValidationError{Err: enformion.ErrInvalidRequest}, want: "enformion: invalid request"},
		{name: "nil sentinel", err: &enformion.ValidationError{Field: "x", Reason: "y"}, want: "enformion: validation failed: x: y"},
		{name: "zero", err: &enformion.ValidationError{}, want: "enformion: validation failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
			assert.Equal(t, tt.err.Err, tt.err.Unwrap())
		})
	}
	ve := &enformion.ValidationError{Err: enformion.ErrInvalidRequest, Field: "Phone", Reason: "blank"}
	require.ErrorIs(t, fmt.Errorf("wrapped: %w", ve), enformion.ErrInvalidRequest)
	require.NotErrorIs(t, ve, enformion.ErrInvalidConfig)
}

func TestErrorsFakeCallErrors(t *testing.T) {
	fake := &enformion.CallError{Kind: enformion.FailureRateLimited, StatusCode: http.StatusTooManyRequests}
	var err error = fake
	require.ErrorIs(t, err, enformion.ErrRateLimited)
	require.NoError(t, fake.Unwrap())
	assert.Equal(t, enformion.ProviderDiagnostics{}, fake.Diagnostics())

	wrapped := fmt.Errorf("handler: %w", err)
	assert.Same(t, fake, callError(t, wrapped))
	require.ErrorIs(t, wrapped, enformion.ErrRateLimited)
}

// TestErrorsCanceledCause proves a real canceled call keeps the context error
// as its cause and its text is the bare kind.
func TestErrorsCanceledCause(t *testing.T) {
	ct := &testutil.CountingTransport{Next: testutil.StaticTransport(http.StatusOK, emptyEnvelope)}
	c := newTransportClient(t, ct)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := c.EnrichContact(ctx, validRequest())
	ce := callError(t, err)
	assert.Equal(t, enformion.FailureCanceled, ce.Kind)
	assert.Zero(t, ce.StatusCode)
	assert.Equal(t, context.Canceled, ce.Unwrap())
	assert.Equal(t, "enformion: canceled", err.Error())
	assert.Zero(t, ct.Attempts())
}
