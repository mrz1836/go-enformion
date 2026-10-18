package enformion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Sentinel errors. Match them with errors.Is.
//
// ErrInvalidConfig and ErrInvalidRequest are local: an error matching either
// means no request was sent. Every other sentinel describes a call that was
// attempted, and for those the provider may already have processed, and billed,
// the request.
var (
	// ErrInvalidConfig reports an invalid NewClient argument or option, or a
	// Client that was not created by NewClient. No request was sent.
	ErrInvalidConfig = errors.New("enformion: invalid configuration")

	// ErrInvalidRequest reports a request that fails local validation, or a nil
	// context. No request was sent.
	ErrInvalidRequest = errors.New("enformion: invalid request")

	// ErrUnauthorized reports an HTTP 401 or 403 response: the provider refused
	// the credentials or the account's access.
	ErrUnauthorized = errors.New("enformion: unauthorized")

	// ErrRateLimited reports an HTTP 429 response, which the provider may use for
	// a rate limit or an exhausted monthly quota. The SDK never retries.
	ErrRateLimited = errors.New("enformion: rate limited")

	// ErrRejected reports a refused request: an HTTP 4xx status other than 401,
	// 403 and 429, or an HTTP 200 response whose envelope says isError: true.
	// A 404 is a rejection, never a "no match".
	ErrRejected = errors.New("enformion: rejected by provider")

	// ErrUpstream reports an HTTP 5xx response.
	ErrUpstream = errors.New("enformion: provider server error")

	// ErrUnexpectedResponse reports a response this SDK cannot interpret: a
	// status other than 200 outside the 4xx and 5xx ranges (including a 1xx,
	// another 2xx, a redirect, which is never followed, and anything from 600
	// on), a body that is not a JSON object, an isError flag that is missing or
	// not a boolean, a Person Search body without its persons member, or a
	// record whose only data is under keys this SDK does not decode: a Contact
	// Enrichment person, its name, or one of its address, phone or email
	// entries, or a Person Search person or one of its names, addresses,
	// phones, emails, locations, relatives, associates, dates of birth or
	// death records. The provider may already have processed and billed the
	// request.
	ErrUnexpectedResponse = errors.New("enformion: unexpected response")

	// ErrMalformedResponse reports an HTTP 200 body that is not valid UTF-8 or
	// valid JSON, has data after the top-level value, repeats a key, or holds a
	// recognized field of the wrong JSON type. The provider may already have
	// processed and billed the request.
	ErrMalformedResponse = errors.New("enformion: malformed response")

	// ErrResponseTooLarge reports an HTTP 200 body larger than the configured
	// limit (see WithMaxResponseBytes), a decoded list with more than 1,000
	// entries, an SDK safety bound (the provider documents at most five per
	// Contact Enrichment person list), or a Person Search body that would
	// decode to more than four times its size plus 1 MiB, another SDK bound
	// that keeps a hostile response from costing many times its size in
	// memory. The provider may already have processed and billed the request.
	ErrResponseTooLarge = errors.New("enformion: response too large")

	// ErrTransport reports a failure in transit, such as DNS, connection, TLS or
	// proxy errors, a reset, or a truncated body. The request may or may not have
	// reached the provider, which may have processed and billed it.
	ErrTransport = errors.New("enformion: transport failure")

	// ErrTimeout reports that the SDK's own timeout (WithTimeout) or an injected
	// http.Client's Timeout expired, or that the network reported a timeout. It
	// also matches context.DeadlineExceeded, so check it first to tell it apart
	// from the caller's own deadline. The provider may already have processed
	// and billed the request.
	ErrTimeout = errors.New("enformion: timeout")
)

// Static causes kept inside a CallError. They replace standard-library errors,
// whose messages can quote URLs, header bytes or response content.
var (
	errResponseTooLarge = errors.New("enformion: response body exceeds the size limit")
	errBuildRequest     = errors.New("enformion: cannot build the HTTP request")
	errStatus           = errors.New("enformion: unexpected HTTP status")
	errInvalidUTF8      = errors.New("enformion: response is not valid UTF-8")
	errInvalidJSON      = errors.New("enformion: response is not valid JSON")
	errTrailingData     = errors.New("enformion: response has data after the top-level value")
	errDuplicateKey     = errors.New("enformion: response repeats an object key")
	errNotObject        = errors.New("enformion: response is not a JSON object")
	errWrongType        = errors.New("enformion: response field has the wrong JSON type")
	errIsErrorFlag      = errors.New("enformion: response isError flag is missing or not a boolean")
	errProviderError    = errors.New("enformion: response envelope reports isError: true")
	errUnrecognizedData = errors.New("enformion: response data is only under unrecognized keys")
	errTooManyEntries   = errors.New("enformion: response collection exceeds the entry limit")
	errDecodeBudget     = errors.New("enformion: response decodes to more than the decode budget")
	errMissingMember    = errors.New("enformion: response lacks a required member")

	// Transport classes. The original error is discarded, because standard
	// library errors can quote server bytes such as a malformed status line.
	errTransportDNS    = errors.New("enformion: transport: DNS lookup failed")
	errTransportProxy  = errors.New("enformion: transport: proxy connection failed")
	errTransportDial   = errors.New("enformion: transport: connection failed")
	errTransportTLS    = errors.New("enformion: transport: TLS failed")
	errTransportReset  = errors.New("enformion: transport: connection reset")
	errTransportClosed = errors.New("enformion: transport: connection closed early")
	errTransportOther  = errors.New("enformion: transport: request failed")
)

// FailureKind classifies a failed call. Each kind matches one target with
// errors.Is (a sentinel of this package, context.Canceled or
// context.DeadlineExceeded), except FailureTimeout, which matches both
// ErrTimeout and context.DeadlineExceeded. The values are short, fixed and free
// of personal data, so they are safe to use as metric labels.
type FailureKind string

// Failure kinds.
const (
	// FailureUnauthorized matches ErrUnauthorized (HTTP 401 or 403).
	FailureUnauthorized FailureKind = "unauthorized"

	// FailureRateLimited matches ErrRateLimited (HTTP 429).
	FailureRateLimited FailureKind = "rate_limited"

	// FailureRejected matches ErrRejected (another 4xx, or isError: true).
	FailureRejected FailureKind = "rejected"

	// FailureUpstream matches ErrUpstream (HTTP 5xx).
	FailureUpstream FailureKind = "upstream"

	// FailureUnexpectedResponse matches ErrUnexpectedResponse.
	FailureUnexpectedResponse FailureKind = "unexpected_response"

	// FailureMalformedResponse matches ErrMalformedResponse.
	FailureMalformedResponse FailureKind = "malformed_response"

	// FailureResponseTooLarge matches ErrResponseTooLarge.
	FailureResponseTooLarge FailureKind = "response_too_large"

	// FailureTransport matches ErrTransport.
	FailureTransport FailureKind = "transport"

	// FailureTimeout matches ErrTimeout, and also context.DeadlineExceeded.
	FailureTimeout FailureKind = "timeout"

	// FailureCanceled means the caller's context was canceled. It matches
	// context.Canceled.
	FailureCanceled FailureKind = "canceled"

	// FailureDeadlineExceeded means the caller's context deadline expired. It
	// matches context.DeadlineExceeded, but not ErrTimeout.
	FailureDeadlineExceeded FailureKind = "deadline_exceeded"
)

// ValidationError describes a local validation failure: nothing was sent. Err
// is ErrInvalidConfig or ErrInvalidRequest. Field names the argument, option or
// request field, with a list index where it has one, such as "DOB",
// "Addresses[1].County" or "keyPassword", and Reason is a short fixed code,
// such as "invalid_format". Neither ever contains the rejected value.
type ValidationError struct {
	Err    error
	Field  string
	Reason string
}

// Error returns the sentinel's message followed by the field and the reason,
// such as "enformion: invalid request: DOB: invalid_format".
func (e *ValidationError) Error() string {
	if e == nil {
		return "enformion: <nil ValidationError>"
	}
	var b strings.Builder
	if e.Err != nil {
		b.WriteString(e.Err.Error())
	} else {
		b.WriteString("enformion: validation failed")
	}
	for _, part := range [...]string{e.Field, e.Reason} {
		if part != "" {
			b.WriteString(": ")
			b.WriteString(part)
		}
	}
	return b.String()
}

// Unwrap returns the sentinel, ErrInvalidConfig or ErrInvalidRequest.
func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CallError describes a call that was attempted, or that a canceled or expired
// context stopped before it was sent.
//
// Its Error text is built only from Kind, StatusCode and Field: it never holds
// credentials, request values, URLs, headers, response bodies or provider
// messages, so it is safe to log. GoString and every fmt verb add only the
// provider's RequestID: the cause and the provider diagnostics sit behind a
// function value that fmt and reflection-based dumpers print only as an
// address.
//
// Tests and fakes may build a CallError from its exported fields alone; it then
// matches errors.Is like one the SDK returns.
type CallError struct {
	// Kind classifies the failure.
	Kind FailureKind

	// StatusCode is the HTTP status, or 0 when no response was received.
	StatusCode int

	// RequestID is the provider's requestId from the response envelope, kept
	// only when it is 1 to 128 bytes of visible ASCII.
	RequestID string

	// Field is a dotted JSON path, such as "person.phones[1].isConnected", for
	// type and shape failures. It names a location, never a value.
	Field string

	// private holds the sanitized cause and the provider diagnostics.
	private func() callDetail
}

// callDetail is the part of a CallError that is never printed.
type callDetail struct {
	cause       error
	diagnostics ProviderDiagnostics
}

// newCallError returns a CallError without diagnostics whose cause is reachable
// only through Unwrap.
func newCallError(kind FailureKind, status int, cause error) *CallError {
	return newCallErrorWithDiagnostics(kind, status, cause, ProviderDiagnostics{})
}

// newCallErrorWithDiagnostics returns a CallError whose cause and diagnostics
// are reachable only through Unwrap and Diagnostics.
func newCallErrorWithDiagnostics(kind FailureKind, status int, cause error, diag ProviderDiagnostics) *CallError {
	d := callDetail{cause: cause, diagnostics: diag}
	return &CallError{Kind: kind, StatusCode: status, private: func() callDetail { return d }}
}

// Error returns "enformion: <kind>", followed by " (HTTP <status>)" when a
// status was received and ": <field>" when a field is named, such as
// "enformion: rate_limited (HTTP 429)".
func (e *CallError) Error() string {
	if e == nil {
		return "enformion: <nil CallError>"
	}
	var b strings.Builder
	b.WriteString("enformion: ")
	if e.Kind != "" {
		b.WriteString(string(e.Kind))
	} else {
		b.WriteString("call_failed")
	}
	if e.StatusCode != 0 {
		b.WriteString(" (HTTP ")
		b.WriteString(strconv.Itoa(e.StatusCode))
		b.WriteString(")")
	}
	if e.Field != "" {
		b.WriteString(": ")
		b.WriteString(e.Field)
	}
	return b.String()
}

// Unwrap returns the sanitized cause: a context error or a fixed error of this
// package. It is never a *url.Error, a JSON decoding error or anything else
// that could quote a URL, a header or response content. It is nil for a
// CallError built from its exported fields.
func (e *CallError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.detail().cause
}

// Is reports whether target is the sentinel for e.Kind. A timeout also matches
// context.DeadlineExceeded, a caller deadline matches context.DeadlineExceeded
// and a cancellation matches context.Canceled.
func (e *CallError) Is(target error) bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case FailureUnauthorized:
		return target == ErrUnauthorized
	case FailureRateLimited:
		return target == ErrRateLimited
	case FailureRejected:
		return target == ErrRejected
	case FailureUpstream:
		return target == ErrUpstream
	case FailureUnexpectedResponse:
		return target == ErrUnexpectedResponse
	case FailureMalformedResponse:
		return target == ErrMalformedResponse
	case FailureResponseTooLarge:
		return target == ErrResponseTooLarge
	case FailureTransport:
		return target == ErrTransport
	case FailureTimeout:
		return target == ErrTimeout || target == context.DeadlineExceeded
	case FailureCanceled:
		return target == context.Canceled
	case FailureDeadlineExceeded:
		return target == context.DeadlineExceeded
	}
	return false
}

// GoString prints the exported metadata only, so that %#v cannot reach the
// cause or the provider diagnostics.
func (e *CallError) GoString() string {
	if e == nil {
		return "(*enformion.CallError)(nil)"
	}
	return fmt.Sprintf("&enformion.CallError{Kind:%q, StatusCode:%d, RequestID:%q, Field:%q}",
		string(e.Kind), e.StatusCode, e.RequestID, e.Field)
}

// Diagnostics returns a deep copy of the provider's message, inputErrors and
// warnings for this failure. It is empty unless the Client was created with
// WithProviderDiagnostics. Provider text may echo the request's personal data:
// inspect it, but think before logging or storing it.
func (e *CallError) Diagnostics() ProviderDiagnostics {
	if e == nil {
		return ProviderDiagnostics{}
	}
	return e.detail().diagnostics.clone()
}

func (e *CallError) detail() callDetail {
	if e.private == nil {
		return callDetail{}
	}
	return e.private()
}

// clone returns a deep copy of d, with empty lists as nil.
func (d ProviderDiagnostics) clone() ProviderDiagnostics {
	return ProviderDiagnostics{
		Message:     d.Message,
		InputErrors: cloneRaw(d.InputErrors),
		Warnings:    cloneRaw(d.Warnings),
	}
}

// contextFailure returns the CallError for a caller context that is done, with
// the context's error as its cause.
func contextFailure(err error, status int) *CallError {
	kind := FailureCanceled
	if errors.Is(err, context.DeadlineExceeded) {
		kind = FailureDeadlineExceeded
		err = context.DeadlineExceeded
	} else {
		err = context.Canceled
	}
	return newCallError(kind, status, err)
}

// invalidConfig returns a *ValidationError matching ErrInvalidConfig.
func invalidConfig(field, reason string) error {
	return &ValidationError{Err: ErrInvalidConfig, Field: field, Reason: reason}
}

// invalidRequest returns a *ValidationError matching ErrInvalidRequest.
func invalidRequest(field, reason string) error {
	return &ValidationError{Err: ErrInvalidRequest, Field: field, Reason: reason}
}

// cloneRaw deep-copies a list of raw JSON values, returning nil when it is empty.
func cloneRaw(in []json.RawMessage) []json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]json.RawMessage, len(in))
	for i, v := range in {
		out[i] = slices.Clone(v)
	}
	return out
}
