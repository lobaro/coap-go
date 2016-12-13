package coap

import "io"

type Response struct {
	Status     string // e.g. "2.05 Content"
	StatusCode int    // e.g. 69 - which is the int representation of 2.05

	// Body represents the response body.
	//
	// The coap Client and Transport guarantee that Body is always
	// non-nil, even on responses without a body or responses with
	// a zero-length body. It is the caller's responsibility to
	// close Body. The default CoAP client's Transport does not
	// attempt to reuse connections ("keep-alive") unless the Body
	// is read to completion and is closed.
	//
	// The Body is automatically dechunked if the server replied
	// with a "chunked" Transfer-Encoding.
	// See: RFC 7959 (Block-wise transfers in CoAP)
	Body io.ReadCloser

	// Request is the request that was sent to obtain this Response.
	// Request's Body is nil (having already been consumed).
	// This is only populated for Client requests.
	Request *Request
}
