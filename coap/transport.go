package coap

import (
	"errors"
)

// Transport that delegates to other transports based
// on the request URL scheme
type Transport struct {
	TransUart RoundTripper
}

func (t *Transport) RoundTrip(req *Request) (*Response, error) {

	if req.URL.Scheme == UartScheme {
		return t.TransUart.RoundTrip(req)
	}

	return nil, errors.New("Unsupported scheme: " + req.URL.Scheme)
}

var DefaultTransport RoundTripper = &Transport{
	TransUart: NewTransportUart(),
}
