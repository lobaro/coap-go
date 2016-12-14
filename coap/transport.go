package coap

import (
	"errors"
)

// Transport that delegates to other transports based
// on the request URL scheme
type Transport struct {
	TransRs232 RoundTripper
}

func (t *Transport) RoundTrip(req *Request) (*Response, error) {

	if req.URL.Scheme == "coap+rs232" {
		return t.TransRs232.RoundTrip(req)
	}

	return nil, errors.New("Unsupported scheme: " + req.URL.Scheme)
}

var DefaultTransport RoundTripper = &Transport{
	TransRs232: NewTransportRs232(),
}
