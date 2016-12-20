package coap

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Lobaro/coap-go/coapmsg"
	"io"
	"io/ioutil"
	"net/url"
)

// A Request represents a CoAP request received by a server
// or to be sent by a client.
//
// The orientation for the structure is the http.Request to
// make a conversion as easy as possible and to make it more
// easy to understand for developers who are used to http requests
type Request struct {
	// Method specifies the HTTP method (GET, POST, PUT, etc.).
	// For client requests an empty string means GET.
	Method string

	// Confirmable CoAP requests will be confirmed by an ACK
	// from the server.
	Confirmable bool

	// URL specifies either the URI being requested (for server
	// requests) or the URL to access (for client requests).
	//
	// For server requests the URL is parsed from the URI
	// supplied in the CoAP message. (See RFC 7252, Section 6)
	//
	// For client requests, the URL's Host specifies the server to
	// connect to.
	URL *url.URL

	// The protocol version for incoming server requests.
	//
	// For client requests these fields are ignored. The CoAP
	// client code always uses CoAP/1.
	// See the docs on Transport for details.
	Proto        string // "CoAP/1"
	ProtoVersion int    // 1 - encoded as 2 bit field in CoAP messages

	// CoAP Options are like HTTP Headers and used in a similar way
	Options coapmsg.CoapOptions

	// Body is the request's body.
	//
	// For client requests a nil body means the request has no
	// body, such as a GET request. The Client's Transport
	// is responsible for calling the Close method.
	//
	// For server requests the Request Body is always non-nil
	// but will return EOF immediately when no body is present.
	// The Server will close the request body. The Serve****
	// Handler does not need to.
	Body io.ReadCloser

	// Cancel is an optional channel whose closure indicates that the client
	// request should be regarded as canceled. Not all implementations of
	// RoundTripper may support Cancel.
	//
	// For server requests, this field is not applicable.
	//
	// Deprecated: Use the Context and WithContext methods
	// instead. If a Request's Cancel field and context are both
	// set, it is undefined whether Cancel is respected.
	Cancel <-chan struct{}

	// ctx is either the client or server context. It should only
	// be modified via copying the whole Request using WithContext.
	// It is unexported to prevent people from using Context wrong
	// and mutating the contexts held by callers of the same request.
	ctx context.Context
}

// NewRequest returns a new Request given a method, URL, and optional body.
//
// If the provided body is also an io.Closer, the returned
// Request.Body is set to body and will be closed by the Client
// methods Do, Post, and PostForm, and Transport.RoundTrip.
//
// NewRequest returns a Request suitable for use with Client.Do or
// Transport.RoundTrip.
// To create a request for use with testing a Server Handler use either
// ReadRequest or manually update the Request fields. See the Request
// type's documentation for the difference between inbound and outbound
// request fields.
func NewRequest(method, urlStr string, body io.Reader) (*Request, error) {
	if method == "" {
		// We document that "" means "GET" for Request.Method, and people have
		// relied on that from NewRequest, so keep that working.
		// We still enforce validMethod for non-empty methods.
		method = "GET"
	}
	if !ValidMethod(method) {
		return nil, fmt.Errorf("coap: invalid method %q", method)
	}

	if body == nil {
		body = ioutil.NopCloser(&bytes.Buffer{})
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	rc, ok := body.(io.ReadCloser)
	if !ok {
		rc = ioutil.NopCloser(body)
	}

	// The host's colon:port should be normalized. See Issue 14836.
	u.Host = removeEmptyPort(u.Host)
	req := &Request{
		Method:       method,
		Confirmable:  true,
		URL:          u,
		Proto:        "CoAP/1",
		ProtoVersion: 1,
		Options:      make(coapmsg.CoapOptions),
		Body:         rc,
	}

	return req, nil
}

// Context returns the request's context. To change the context, use
// WithContext.
//
// The returned context is always non-nil; it defaults to the
// background context.
//
// For outgoing client requests, the context controls cancelation.
//
// For incoming server requests, the context is canceled when the
// ServeHTTP method returns. For its associated values, see
// ServerContextKey and LocalAddrContextKey.
func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

// WithContext returns a shallow copy of r with its context changed
// to ctx. The provided ctx must be non-nil.
func (r *Request) WithContext(ctx context.Context) *Request {
	if ctx == nil {
		panic("nil context")
	}
	r2 := new(Request)
	*r2 = *r
	r2.ctx = ctx
	return r2
}

func (r *Request) closeBody() {
	if r.Body != nil {
		r.Body.Close()
	}
}

var validMethods = []string{"GET", "POST", "PUT", "DELETE"}

func ValidMethod(method string) bool {
	for _, m := range validMethods {
		if method == m {
			return true
		}
	}

	return false
}

type badStringError struct {
	what string
	str  string
}

func (e *badStringError) Error() string {
	return fmt.Sprintf("%s %q", e.what, e.str)
}
