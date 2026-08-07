package security

import (
	"io"
	"net/http"
)

// ReadResponseBounded ports readResponseText (core.mjs:118-144): fails with
// Error{Code:"response_too_large", Message:"The provider response exceeded the
// safe size limit."} when resp.ContentLength (the parsed Content-Length header)
// exceeds maxBytes, or when the body read through a maxBytes+1 limited reader
// exceeds maxBytes — checked twice. Does not close resp.Body. Non-oversize read
// errors are returned unwrapped.
func ReadResponseBounded(resp *http.Response, maxBytes int64) ([]byte, error) {
	if resp.ContentLength > maxBytes {
		return nil, &Error{Code: "response_too_large", Message: "The provider response exceeded the safe size limit."}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, &Error{Code: "response_too_large", Message: "The provider response exceeded the safe size limit."}
	}
	return data, nil
}
