package coap

import (
	"errors"
	"time"
)

const ACK_RANDOM_FACTOR = 1.5
const MAX_RETRANSMIT = 4

var AckTimeout = 10 * time.Second // Default 2 Seconds

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

// For a new Confirmable message, the initial timeout is set
// to a random duration (often not an integral number of seconds)
// between AckTimeout and (AckTimeout * ACK_RANDOM_FACTOR)
//
// When the timeout is triggered and the retransmission counter is
// less than MAX_RETRANSMIT, the message is retransmitted, the
// retransmission counter is incremented, and the timeout is doubled.
func ackTimeout() time.Duration {
	// TODO: Add random factor
	return time.Duration(float64(AckTimeout) * ACK_RANDOM_FACTOR)
}
