package coap

import (
	"errors"
)

type Transport struct {
}

func (*Transport) RoundTrip(*Request) (*Response, error) {
	return nil, errors.New("Invalid transport")
}

var DefaultTransport RoundTripper = &Transport{}
