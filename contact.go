package enformion

import "context"

// Contact Enrichment operation constants.
const (
	contactEnrichPath       = "/Contact/Enrich"
	contactEnrichSearchType = "DevAPIContactEnrich"
)

// EnrichContact sends one subject to the EnformionGO Contact Enrichment API,
// POST https://devapi.enformion.com/Contact/Enrich, and returns the provider's
// top person match.
//
// The steps, in order:
//   - check the Client, ctx and every request field, then the search
//     categories, failing with a *ValidationError and sending nothing
//   - fail with a *CallError (canceled or deadline_exceeded), sending nothing,
//     when ctx is already done
//   - make exactly one POST attempt, bounded by the Client's timeout and ctx,
//     including the read of the whole response body
//   - classify the status, then strictly decode an HTTP 200 body
//
// Parameters:
//   - ctx: cancels the call and bounds it with its deadline
//   - req: the subject; see EnrichContactRequest for the rules
//
// Returns the result, or an error that is a *ValidationError (nothing was
// sent) or a *CallError. A nil Person in the result means the response carried
// no person data; it is not proof that no match exists. DecodeEnrichContact
// decodes a kept response body the same way.
//
// Side effects: one live, potentially billable call. The provider's pricing
// pages say it charges per successful match. The SDK never retries, follows a
// redirect, polls or makes a follow-on call. After a timeout, cancellation,
// transport failure or unreadable response, the provider may already have
// processed and billed the request: the SDK cannot tell, so repeating the call
// may be billed again.
func (c *Client) EnrichContact(ctx context.Context, req EnrichContactRequest) (*EnrichContactResult, error) {
	if c == nil || c.send == nil {
		return nil, invalidConfig("Client", reasonNotInitialized)
	}
	if ctx == nil {
		return nil, invalidRequest("ctx", reasonNil)
	}
	body, err := req.encode()
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, contextFailure(err, 0)
	}
	ex, err := c.send(ctx, &call{
		path:       contactEnrichPath,
		searchType: contactEnrichSearchType,
		sessionID:  req.SessionID,
		body:       body,
	})
	if err != nil {
		return nil, err
	}
	return decodeEnrichContact(ex, c.diagnostics)
}

// DecodeEnrichContact decodes body, the bytes of an HTTP 200 Contact
// Enrichment response that the caller kept, exactly as EnrichContact decodes
// the response it reads, and returns the same result or the same *CallError.
// It makes no call and needs no Client or credentials, and nothing about the
// request.
//
// Errors are *CallError values with StatusCode 200, as a call's are for an
// HTTP 200 response: ErrMalformedResponse, ErrUnexpectedResponse,
// ErrRejected (isError: true) and ErrResponseTooLarge, the last also for a
// body over 16 MiB, the most any call reads. They carry no provider
// diagnostics, as from a Client created without WithProviderDiagnostics; a
// result carries its diagnostics, as every result does.
//
// body must be the whole body of an HTTP 200 response, as the call read it,
// after any decompression; a call that received any other status never
// decoded its body. DecodeEnrichContact never modifies body, and the result
// shares no memory with it.
func DecodeEnrichContact(body []byte) (*EnrichContactResult, error) {
	ex, err := storedExchange(body)
	if err != nil {
		return nil, err
	}
	return decodeEnrichContact(ex, false) // errors keep no provider diagnostics
}
