package coap

import (
	"bytes"
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/Sirupsen/logrus"
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

	// We can only encode % in IPv6 hosts - normally only used for zoneIDs
	uri, err = url.Parse("tcp://[%25dev%25tty]")
	expectedHost := "[%dev%tty]"
	if err != nil && err.Error() != "parse "+UartScheme+"://%2Fdev%2Ftty: invalid URL escape \"%2F\"" {
		t.Error("Unexpected error:", err)
	} else if uri.Host != expectedHost {
		t.Error("Expected Host to be", expectedHost, "but was", uri.Host)
	}

}

// A test should not leave any bytes on the wire uninterpreted
func ValidateRemainingBytes(t *testing.T, conn *TestConnector) {
	var writtenBytes = make([]byte, 500)
	//n, err := conn.SendBuf.Read(writtenBytes)
	n := conn.Out.Len()

	/*
		if err != nil && err != io.EOF {
			t.Error(err)
		}*/

	if n > 0 {
		t.Logf("Unhandled Transport SendBuf %d bytes: %v", n, writtenBytes[0:n])
		t.Error("SendBuf is not empty - handle all bytes in test")
	}

	var readBytes = make([]byte, 500)
	//n, err = conn.ReceiveBuf.Read(readBytes)
	n = conn.In.Len()

	/*
		if err != nil && err != io.EOF {
			t.Error(err)
		}*/

	if n > 0 {
		t.Logf("Unhandled Transport ReceiveBuf %d bytes: %v", n, readBytes[0:n])
		t.Error("ReceiveBuf is not empty - handle all bytes in test")
	}
}

func TestRequestResponsePiggyback(t *testing.T) {
	trans := NewTransportUart()
	testCon := NewTestConnector()
	trans.Connecter = testCon

	testCon.Connect("ignored")

	req, err := NewRequest("GET", "coap+uart://any/foo", nil)

	if err != nil {
		t.Error(err)
	}

	// Deliver expected network traffic async.
	go func() {
		// Check outgoing message
		msg, err := testCon.WaitForSendMessage(1 * time.Second)
		if err != nil {
			t.Error(err)
		}
		if msg.PathString() != "foo" {
			t.Errorf("Expected empty PathString but was %s", msg.PathString())
		}

		// Send ack
		ack := coapmsg.NewAck(msg.MessageID)
		ack.Code = coapmsg.Content // For piggyback response. Default Empty would be postponed
		ack.Token = msg.Token
		ack.Payload = []byte("test")
		testCon.FakeReceiveMessage(ack)
	}()

	// Shorter timeout
	ctxWithTimeout, _ := context.WithTimeout(req.Context(), time.Second)
	req = req.WithContext(ctxWithTimeout)
	res, err := trans.RoundTrip(req)

	if err != nil {
		t.Error(err)
	}

	if res != nil {
		body := bytes.Buffer{}
		body.ReadFrom(res.Body)
		t.Logf("Response: [%s] %s", coapmsg.COAPCode(res.StatusCode).String(), body.String())
		if body.String() != "test" {
			t.Error("Expected response payload 'test' but got " + body.String())
		}
		if res.StatusCode != coapmsg.Content.Number() {
			t.Errorf("Expected response code %d got %d", coapmsg.Content.Number(), res.StatusCode)
		}
	}

	ValidateRemainingBytes(t, testCon)
}

// This sometime fails.
func TestMany(t *testing.T) {
	/*for i := 0; i < 10; i++ {
		go func() {
			TestRequestResponsePostponed(t)
		}()
	}*/

	for i := 0; i < 15; i++ {
		TestRequestResponsePostponed(t)
		t.Logf("Done test run #%d", i)
	}

	//time.Sleep(5 * time.Second)
}

func TestRequestResponsePostponed(t *testing.T) {
	trans := NewTransportUart()
	testCon := NewTestConnector()
	trans.Connecter = testCon

	testCon.Connect("ignored")

	req, err := NewRequest("GET", "coap+uart://any/foo", nil)

	if err != nil {
		t.Error(err)
	}

	// Deliver expected network traffic async.
	asyncDoneChan := make(chan bool)
	go func() {
		// Check outgoing message
		msg, err := testCon.WaitForSendMessage(3 * time.Second)
		if err != nil {
			t.Error(err)
			asyncDoneChan <- true
			return
		}
		if msg.PathString() != "foo" {
			t.Errorf("Expected empty PathString but was %s", msg.PathString())
		}

		// Send ack
		ack := coapmsg.NewAck(msg.MessageID)
		ack.Code = coapmsg.Empty // For postponed response.
		logrus.Info("Fake Receive ACK")
		testCon.FakeReceiveMessage(ack)

		// let some time pass and send response
		time.Sleep(10 * time.Millisecond)
		res := coapmsg.NewMessage()
		res.Type = coapmsg.Confirmable
		res.MessageID = msg.MessageID
		res.Token = msg.Token
		res.Code = coapmsg.Content
		res.Payload = []byte("test")
		logrus.Info("Fake Receive CON")
		testCon.FakeReceiveMessage(res)

		// Expect an acknowledgment for our CON
		msg, err = testCon.WaitForSendMessage(3 * time.Second)

		if err != nil {
			t.Error(err)
			asyncDoneChan <- true
			return
		}
		if msg.Type != coapmsg.Acknowledgement {
			t.Errorf("Expected Acknowledgement but got %s", msg.Type.String())
		}

		asyncDoneChan <- true

	}()

	// Shorter timeout
	ctxWithTimeout, _ := context.WithTimeout(req.Context(), 10*time.Second)
	req = req.WithContext(ctxWithTimeout)
	res, err := trans.RoundTrip(req)

	if err != nil {
		t.Error(err)
	}

	if res != nil {
		body := bytes.Buffer{}
		body.ReadFrom(res.Body)
		t.Logf("Response: [%s] %s", coapmsg.COAPCode(res.StatusCode).String(), body.String())
		if body.String() != "test" {
			t.Error("Expected response payload 'test' but got " + body.String())
		}
		if res.StatusCode != coapmsg.Content.Number() {
			t.Errorf("Expected response code %d got %d", coapmsg.Content.Number(), res.StatusCode)
		}
	}

	select {
	case <-asyncDoneChan:
		t.Log("Test Done.")
	case <-time.After(10 * time.Second):
		t.Error("Test Failed: Timeout")
	}
	ValidateRemainingBytes(t, testCon)
}
