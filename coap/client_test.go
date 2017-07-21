package coap

import (
	"errors"
	"testing"
)

type recordingTransport struct {
	req *Request
}

func TestChans(t *testing.T) {
	var ch chan bool
	ch = make(chan bool, 1) // Read on null chan would fail or default with case.

	ch <- true
	close(ch)

	// Receive value from before close works
	b, ok := <-ch
	t.Logf("Read on closed chan: result = %v, OK = %v", b, ok)

	select {
	case b, ok := <-ch:
		t.Logf("Read on closed chan: result = %v, OK = %v", b, ok)
	default:
		t.Logf("Read on closed chan: default!")
	}

	select {
	case b, ok := <-ch:
		t.Logf("Read on closed chan: result = %v, OK = %v", b, ok)
	}
}

func (t *recordingTransport) RoundTrip(req *Request) (resp *Response, err error) {
	t.req = req
	return nil, errors.New("dummy impl")
}

func TestGetRequestFormat(t *testing.T) {
	//defer afterTest(t)
	tr := &recordingTransport{}
	client := &Client{Transport: tr}
	url := "http://dummy.faketld/"
	_, err := client.Get(url) // Note: doesn't hit network
	if err != nil && err.Error() != "dummy impl" {
		t.Error(err)
	}

	if tr.req.Method != "GET" {
		t.Errorf("expected method %q; got %q", "GET", tr.req.Method)
	}
	if tr.req.URL.String() != url {
		t.Errorf("expected URL %q; got %q", url, tr.req.URL.String())
	}
	if tr.req.Options == nil {
		t.Errorf("expected non-nil request Options")
	}
}
