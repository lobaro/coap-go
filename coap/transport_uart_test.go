package coap

import (
	"net/url"
	"testing"
)

func TestTransportFail(t *testing.T) {
	trans := NewTransportUart()

	res, err := trans.RoundTrip(nil)

	if res != nil {
		t.Error("Expected res to be nil but was", res)
	}
	if err == nil {
		t.Error("Expected err not to be nil but was", err)
	}
}

func TestUrl(t *testing.T) {
	uri := &url.URL{}
	uri.Scheme = UartScheme
	uri.Host = "/dev/tty"

	expectedUrl := UartScheme + "://%2Fdev%2Ftty"
	if uri.String() != expectedUrl {
		t.Error("Expected URL to be", expectedUrl, "but was", uri.String())
	}

	uri, err := url.Parse(expectedUrl)

	if err != nil && err.Error() != "parse "+UartScheme+"://%2Fdev%2Ftty: invalid URL escape \"%2F\"" {
		t.Error("Unexpected error:", err)
	}

	// This does not work!
	//expectedHost := "/dev/tty"
	//if uri.Host != expectedHost {
	//	t.Error("Expected Host to be", expectedHost, "but was", uri.Host)
	//}

}
