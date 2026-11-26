package enformion

import "context"

// personSearchPath is the Person Search route. Its galaxy-search-type value is
// the request's Mode.
const personSearchPath = "/PersonSearch"

// SearchPersons sends one Person Search page request to the EnformionGO
// Person Search API, POST https://devapi.enformion.com/PersonSearch, with the
// request's Mode as the search type, and returns the provider's candidates
// for that page.
//
// The steps, in order:
//   - check the Client, ctx and every request field, then the criteria,
//     failing with a *ValidationError and sending nothing
//   - fail with a *CallError (canceled or deadline_exceeded), sending nothing,
//     when ctx is already done
//   - make exactly one POST attempt, bounded by the Client's timeout and ctx,
//     including the read of the whole response body, within the response
//     limit (16 MiB unless WithMaxResponseBytes was given)
//   - classify the status, then strictly decode an HTTP 200 body
//
// Parameters:
//   - ctx: cancels the call and bounds it with its deadline
//   - req: the mode, criteria and page; see PersonSearchRequest for the rules
//
// Returns the result, or an error that is a *ValidationError (nothing was
// sent) or a *CallError. A result without Persons means the response carried
// no candidates; it is not proof that no match exists. The candidates are the
// provider's, in its order: the SDK never selects, ranks or merges them.
// DecodeSearchPersons decodes a kept response body the same way.
//
// Side effects: one live, potentially billable call for one page. Each page,
// and each change of mode, is a separate call that may be billed: the SDK
// never fetches a next page, upgrades a teaser search, retries, follows a
// redirect, polls or makes a follow-on call. After a timeout, cancellation,
// transport failure or unreadable response, the provider may already have
// processed and billed the request: the SDK cannot tell, so repeating the
// call may be billed again.
func (c *Client) SearchPersons(ctx context.Context, req PersonSearchRequest) (*PersonSearchResult, error) {
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
		path:       personSearchPath,
		searchType: string(req.Mode),
		sessionID:  req.SessionID,
		body:       body,
		maxBytes:   c.searchMaxBytes,
	})
	if err != nil {
		return nil, err
	}
	return decodeSearchPersons(ex, c.diagnostics)
}

// DecodeSearchPersons decodes body, the bytes of an HTTP 200 Person Search
// response that the caller kept, exactly as SearchPersons decodes the response
// it reads, and returns the same result or the same *CallError. It makes no
// call and needs no Client or credentials, and nothing about the request.
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
// decoded its body. DecodeSearchPersons never modifies body, and the result
// shares no memory with it.
func DecodeSearchPersons(body []byte) (*PersonSearchResult, error) {
	ex, err := storedExchange(body)
	if err != nil {
		return nil, err
	}
	return decodeSearchPersons(ex, false) // errors keep no provider diagnostics
}
