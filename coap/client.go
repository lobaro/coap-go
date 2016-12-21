package coap

import (
	"errors"
	"fmt"
	"github.com/Lobaro/coap-go/coapmsg"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// RoundTripper is an interface representing the ability to execute a
// single HTTP transaction, obtaining the Response for a given Request.
//
// A RoundTripper must be safe for concurrent use by multiple
// goroutines.
type RoundTripper interface {
	// RoundTrip executes a single HTTP transaction, returning
	// a Response for the provided Request.
	//
	// RoundTrip should not attempt to interpret the response. In
	// particular, RoundTrip must return err == nil if it obtained
	// a response, regardless of the response's HTTP status code.
	// A non-nil err should be reserved for failure to obtain a
	// response. Similarly, RoundTrip should not attempt to
	// handle higher-level protocol details such as redirects,
	// authentication, or cookies.
	//
	// RoundTrip should not modify the request, except for
	// consuming and closing the Request's Body.
	//
	// RoundTrip must always close the body, including on errors,
	// but depending on the implementation may do so in a separate
	// goroutine even after RoundTrip returns. This means that
	// callers wanting to reuse the body for subsequent requests
	// must arrange to wait for the Close call before doing so.
	//
	// The Request's URL and Header fields must be initialized.
	RoundTrip(*Request) (*Response, error)
}

// A Client is an HTTP client. Its zero value (DefaultClient) is a
// usable client that uses DefaultTransport.
//
// The Client's Transport typically has internal state (cached TCP
// connections), so Clients should be reused instead of created as
// needed. Clients are safe for concurrent use by multiple goroutines.
//
// A Client is higher-level than a RoundTripper (such as Transport)
// and additionally handles CoAP details such parallel request limit
type Client struct {
	// Transport specifies the mechanism by which individual
	// CoAP requests are made.
	// If nil, DefaultTransport is used.
	Transport RoundTripper

	// Timeout specifies a time limit for requests made by this
	// Client. The timeout includes connection time, any
	// redirects, and reading the response body. The timer remains
	// running after Get, Head, Post, or Do return and will
	// interrupt reading of the Response.Body.
	//
	// A Timeout of zero means no timeout.
	//
	// The Client cancels requests to the underlying Transport
	// using the Request.Cancel mechanism. Requests passed
	// to Client.Do may still set Request.Cancel; both will
	// cancel the request.
	//
	// For compatibility, the Client will also use the deprecated
	// CancelRequest method on Transport if found. New
	// RoundTripper implementations should use Request.Cancel
	// instead of implementing CancelRequest.
	Timeout time.Duration

	// CoAP spcifies the constant NSTART (default = 1) to limit
	// the amount of parallel requests. 0 = no limit.
	// The default client has a value of 1 as proposed by the RFC
	MaxParallelRequests int32
	runningRequests     int32
	mu                  sync.Mutex
}

// DefaultClient is the default Client and is used by Get, Head, and Post.
var DefaultClient = &Client{
	Transport:           DefaultTransport,
	MaxParallelRequests: 1,
}

func Get(url string) (*Response, error) {
	return DefaultClient.Get(url)
}

func Post(url string, bodyType uint16, body io.Reader) (*Response, error) {
	return DefaultClient.Post(url, bodyType, body)
}

func (c *Client) Do(req *Request) (res *Response, err error) {
	c.mu.Lock()
	if c.runningRequests >= c.MaxParallelRequests && c.MaxParallelRequests != 0 {
		c.mu.Unlock()
		return nil, errors.New(fmt.Sprint("MaxParallelRequests exhausted: ", c.MaxParallelRequests))
	}
	c.runningRequests++
	c.mu.Unlock()
	res, err = c.send(req, c.deadline())
	atomic.AddInt32(&c.runningRequests, -1)
	return
}

// Get issues a GET to the specified URL.
//
// When err is nil, resp always contains a non-nil resp.Body.
// Caller should close resp.Body when done reading from it.
//
// To make a request with custom options, use NewRequest and
// DefaultClient.Do.
func (c *Client) Get(url string) (*Response, error) {
	req, err := NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post issues a POST to the specified URL.
//
// Caller should close resp.Body when done reading from it.
//
// If the provided body is an io.Closer, it is closed after the
// request.
//
// To set custom headers, use NewRequest and Client.Do.
func (c *Client) Post(url string, bodyType uint16, body io.Reader) (*Response, error) {
	req, err := NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Options.Set(coapmsg.ContentFormat, coapmsg.OptionValue(bodyType))
	return c.Do(req)
}

func (c *Client) send(req *Request, deadline time.Time) (*Response, error) {

	resp, err := send(req, c.transport(), deadline)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) deadline() time.Time {
	if c.Timeout > 0 {
		return time.Now().Add(c.Timeout)
	}
	return time.Time{}
}

func (c *Client) transport() RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	return DefaultTransport
}

// send issues an CoAP request.
// Caller should close resp.Body when done reading from it.
func send(ireq *Request, rt RoundTripper, deadline time.Time) (*Response, error) {
	req := ireq // req is either the original request, or a modified fork

	if rt == nil {
		req.closeBody()
		return nil, errors.New("coap: no Client.Transport or DefaultTransport")
	}

	if req.URL == nil {
		req.closeBody()
		return nil, errors.New("coap: nil Request.URL")
	}

	// forkReq forks req into a shallow clone of ireq the first
	// time it's called.
	forkReq := func() {
		if ireq == req {
			req = new(Request)
			*req = *ireq // shallow clone
		}
	}

	// Most the callers of send (Get, Post, et al) don't need
	// Options, leaving it uninitialized. We guarantee to the
	// Transport that this has been initialized, though.
	if req.Options == nil {
		forkReq()
		req.Options = make(coapmsg.CoapOptions)
	}

	if !deadline.IsZero() {
		forkReq()
	}
	stopTimer, wasCanceled := setRequestCancel(req, rt, deadline)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		stopTimer()
		if resp != nil {
			log.Printf("RoundTripper returned a response & error; ignoring response")
		}
		return nil, err
	}
	if !deadline.IsZero() {
		resp.Body = &cancelTimerBody{
			stop:           stopTimer,
			rc:             resp.Body,
			reqWasCanceled: wasCanceled,
		}
	}
	return resp, nil
}

func alwaysFalse() bool {
	return false
}

// setRequestCancel sets the Cancel field of req, if deadline is
// non-zero. The RoundTripper's type is used to determine whether the legacy
// CancelRequest behavior should be used.
func setRequestCancel(req *Request, rt RoundTripper, deadline time.Time) (stopTimer func(), wasCanceled func() bool) {
	if deadline.IsZero() {
		return nop, alwaysFalse
	}

	initialReqCancel := req.Cancel // the user's original Request.Cancel, if any

	cancel := make(chan struct{})
	req.Cancel = cancel

	wasCanceled = func() bool {
		select {
		case <-cancel:
			return true
		default:
			return false
		}
	}

	doCancel := func() {
		close(cancel)
	}

	stopTimerCh := make(chan struct{})
	var once sync.Once
	stopTimer = func() {
		once.Do(func() {
			close(stopTimerCh)
		})
	}

	timer := time.NewTimer(deadline.Sub(time.Now()))
	go func() {
		select {
		case <-initialReqCancel:
			doCancel()
		case <-timer.C:
			doCancel()
		case <-stopTimerCh:
			timer.Stop()
		}
	}()

	return stopTimer, wasCanceled
}

// cancelTimerBody is an io.ReadCloser that wraps rc with two features:
// 1) on Read error or close, the stop func is called.
// 2) On Read failure, if reqWasCanceled is true, the error is wrapped and
//    marked as net.Error that hit its timeout.
type cancelTimerBody struct {
	stop           func() // stops the time.Timer waiting to cancel the request
	rc             io.ReadCloser
	reqWasCanceled func() bool
}

func (b *cancelTimerBody) Read(p []byte) (n int, err error) {
	n, err = b.rc.Read(p)
	if err == nil {
		return n, nil
	}
	b.stop()
	if err == io.EOF {
		return n, err
	}
	if b.reqWasCanceled() {
		err = &coapError{
			err:     err.Error() + " (Client.Timeout exceeded while reading body)",
			timeout: true,
		}
	}
	return n, err
}

func (b *cancelTimerBody) Close() error {
	err := b.rc.Close()
	b.stop()
	return err
}
