package rs232

import "testing"

func TestTransportFail(t *testing.T) {
	trans := NewTransport()

	res, err := trans.RoundTrip(nil)

	if res != nil {
		t.Error("Expected res to be nil but was", res)
	}
	if err == nil {
		t.Error("Expected err not to be nil but was", err)
	}
}
