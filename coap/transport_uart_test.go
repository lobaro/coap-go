package coap

import (
	"bytes"
	"context"
	"net/url"
	"sync"
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
func ValidateCleanConnection(t *testing.T, conn *TestConnector) {
	//var writtenBytes = make([]byte, 500)
	//n, err := conn.SendBuf.Read(writtenBytes)
	n := conn.Out.Len()

	/*
		if err != nil && err != io.EOF {
			t.Error(err)
		}*/

	if n > 0 {
		t.Logf("Unhandled Transport SendBuf %d messages", n)
		t.Error("SendBuf is not empty - handle all bytes in test")
	}

	//var readBytes = make([]byte, 500)
	//n, err = conn.ReceiveBuf.Read(readBytes)
	n = conn.In.Len()

	/*
		if err != nil && err != io.EOF {
			t.Error(err)
		}*/

	if n > 0 {
		t.Logf("Unhandled Transport SendBuf %d messages", n)
		msg, _ := coapmsg.ParseMessage(conn.In.packets[0])
		t.Errorf("ReceiveBuf is not empty (%d messages) - handle all bytes in test. conn.In.packets[0]: %v, %v", n, conn.In.packets[0], msg.String())
	}

	if conn.conn.InteractionCount() != 0 {
		t.Errorf("Error: Expected interaction count = 0 but was %d", conn.conn.InteractionCount())
	}

	if !conn.conn.Closed() {
		t.Error("Error: Expected connection to be closed, but it's not")
	}
}

func TestRequestResponsePiggyback(t *testing.T) {
	trans := NewTransportUart()
	testCon := NewTestConnector(t)
	trans.Connecter = testCon

	_, err := testCon.Connect("ignored")
	if err != nil {
		t.Error()
	}

	req, err := NewRequest("GET", "coap+uart://any/foo", nil)

	if err != nil {
		t.Error(err)
	}

	// Deliver expected network traffic async.
	asyncDoneChan := make(chan bool)
	go func() {
		// Check outgoing message
		msg, err := testCon.ServerReceive(5 * time.Second)
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
		err = testCon.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}
		asyncDoneChan <- true
	}()

	// Shorter timeout
	ctxWithTimeout, _ := context.WithTimeout(req.Context(), 5*time.Second)
	req = req.WithContext(ctxWithTimeout)
	res, err := trans.RoundTrip(req)

	if err != nil {
		t.Error(err)
	}

	if res != nil {
		body := bytes.Buffer{}
		_, err := body.ReadFrom(res.Body)
		if err != nil {
			t.Error(err)
		}
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
	ValidateCleanConnection(t, testCon)

	if len(testCon.conn.interactions) != 0 {
		t.Errorf("Interactions not cleaned up! len: %d", len(testCon.conn.interactions))
	}
}

// Ensures everything runs in parallel
// In future we might test this with a single underlying Transport
func TestMany(t *testing.T) {
	var wg sync.WaitGroup
	n := 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			TestRequestResponsePostponed(t)
			TestRequestResponsePiggyback(t)
			TestClientObserve(t)
			wg.Done()
		}()
	}

	wg.Wait()
}

// Fails yet, probably because we do not send responses in the correct order when running parallel
func _TestManyParallel(t *testing.T) {
	var wg sync.WaitGroup
	n := 20
	wg.Add(n)

	trans := NewTransportUart()
	testCon := NewTestConnector(t)
	trans.Connecter = testCon
	_, err := testCon.Connect("ignored")
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < n; i++ {
		go func() {
			RunRequestResponsePostponed(t, trans)
			//TestRequestResponsePiggyback(t)
			wg.Done()
		}()
	}

	wg.Wait()
}

func TestRequestResponsePostponed(t *testing.T) {
	trans := NewTransportUart()
	testCon := NewTestConnector(t)
	trans.Connecter = testCon
	_, err := testCon.Connect("ignored")
	if err != nil {
		t.Error(err)
	}
	RunRequestResponsePostponed(t, trans)
}

func RunRequestResponsePostponed(t *testing.T, trans *TransportUart) {
	testCon := trans.Connecter.(*TestConnector)

	req, err := NewRequest("GET", "coap+uart://any/foo", nil)

	if err != nil {
		t.Error(err)
	}

	// Deliver expected network traffic async.
	asyncDoneChan := make(chan bool)
	go func() {
		// Check outgoing message
		msg, err := testCon.ServerReceive(3 * time.Second)
		if err != nil {
			t.Error(err)
			asyncDoneChan <- true
			return
		}
		if msg.PathString() != "foo" {
			t.Errorf("Error: Expected empty PathString but was %s", msg.PathString())
		}

		// Send ack
		ack := coapmsg.NewAck(msg.MessageID)
		ack.Code = coapmsg.Empty // For postponed response.
		err = testCon.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		// let some time pass and send response
		time.Sleep(10 * time.Millisecond)
		res := coapmsg.NewMessage()
		res.Type = coapmsg.Confirmable
		res.MessageID = msg.MessageID
		res.Token = msg.Token
		res.Code = coapmsg.Content
		res.Payload = []byte("test")
		err = testCon.ServerSend(res)
		if err != nil {
			t.Error(err)
		}

		// Expect an acknowledgment for our CON
		msg, err = testCon.ServerReceive(3 * time.Second)

		if err != nil {
			t.Error(err)
			asyncDoneChan <- true
			return
		}
		if msg.Type != coapmsg.Acknowledgement {
			t.Errorf("Error: Expected Acknowledgement but got %s", msg.Type.String())
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
		_, err = body.ReadFrom(res.Body)
		if err != nil {
			t.Error(err)
		}
		t.Logf("Response: [%s] %s", coapmsg.COAPCode(res.StatusCode).String(), body.String())
		if body.String() != "test" {
			t.Error("Error: Expected response payload 'test' but got " + body.String())
		}
		if res.StatusCode != coapmsg.Content.Number() {
			t.Errorf("Error: Expected response code %d got %d", coapmsg.Content.Number(), res.StatusCode)
		}
	}

	select {
	case <-asyncDoneChan:
		t.Log("Test Done.")
	case <-time.After(10 * time.Second):
		t.Error("Error: Test Failed: Timeout")
	}
	ValidateCleanConnection(t, testCon)

	if len(testCon.conn.interactions) != 0 {
		t.Errorf("Error: Interactions not cleaned up! len: %d", len(testCon.conn.interactions))
	}
}

func NewTestClient(t *testing.T) (*Client, *TestConnector) {
	client := NewClient()
	client.Timeout = 10 * time.Second
	trans := NewTransportUart()
	testCon := NewTestConnector(t)
	trans.Connecter = testCon
	_, err := testCon.Connect("ignored")
	if err != nil {
		t.Error(err)
	}

	client.Transport = trans

	return client, testCon
}

// We do 2 GET requests to /foo and /bar after another
// The answers are send out after both requests reached the server
// The first response is an empty ACK
// The second response is a piggyback response to /foo with content test2
// The third response is a postponed response to /bar with content test1
// Tokens and message ID's are counting up so they are predictable as "1" and "2"
func TestParallelRequests(t *testing.T) {
	client, conn := NewTestClient(t)
	client.Transport.(*TransportUart).TokenGenerator = NewCountingTokenGenerator()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		res, err := client.Get("coap+uart://any/foo")
		if err != nil {
			t.Error(err)
		}

		body := bytes.Buffer{}
		_, err = body.ReadFrom(res.Body)
		if err != nil {
			t.Error(err)
		}
		if body.String() != "test1" {
			t.Errorf("Expected body %s but was %s", "test1", body.String())
		}

		wg.Done()
	}()

	wg.Add(1)
	requestsSend := make(chan bool)
	go func() {
		// Wait for first get to be send
		_, err := conn.ServerReceive(3 * time.Second)
		if err != nil {
			t.Error(err)
		}
		requestsSend <- true
		res, err := client.Get("coap+uart://any/bar")
		if err != nil {
			t.Error(err)
		}

		body := bytes.Buffer{}
		_, err = body.ReadFrom(res.Body)
		if err != nil {
			t.Error(err)
		}
		if body.String() != "test2" {
			t.Errorf("Expected body %s but was %s", "test2", body.String())
		}

		wg.Done()
	}()

	// Deliver expected network traffic async.
	wg.Add(1)
	go func() {
		// Wait till all requests are send
		<-requestsSend

		// Wait for first get to be send
		_, err := conn.ServerReceive(3 * time.Second)
		if err != nil {
			t.Error(err)
		}

		logrus.Info("Start sending responses")

		// Confirm Message 1 as postboned
		ack := coapmsg.NewAck(1)
		err = conn.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		// Confirm Message 2
		ack = coapmsg.NewAck(2)
		ack.Code = coapmsg.Content // For piggyback response. Default Empty would be postponed
		ack.Token = []byte{2}
		ack.Payload = []byte("test2")
		err = conn.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		// Send postponed result for first request
		ack = coapmsg.NewMessage()
		ack.Type = coapmsg.NonConfirmable
		ack.Code = coapmsg.Content // For piggyback response. Default Empty would be postponed
		ack.Token = []byte{1}
		ack.Payload = []byte("test1")
		err = conn.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		wg.Done()
	}()

	if waitTimeout(wg, 10*time.Second) {
		ValidateCleanConnection(t, conn)

		if len(conn.conn.interactions) != 0 {
			t.Errorf("Interactions not cleaned up! len: %d", len(conn.conn.interactions))
		}

		t.Log("Test Done.")
	} else {
		t.Error("Test Failed: Timeout")
	}

}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return true // completed normally
	case <-time.After(timeout):
		return false // timed out
	}
}

// TODO: Test handling of unknown Tokens -> Always send NAK!
func _TestReceiveUnknownToken(t *testing.T) {

}

// TODO: Test Observe scenarios
// 1) When the client receives a Observe response without knowing the token -> send NAK
// 2) Test Observe with 1 or 2 updates
// 3) When the Client times out, send a NAK and tell the server to cancel the observe
func TestClientObserve(t *testing.T) {

	client := NewClient()
	client.Timeout = 10 * time.Second
	trans := NewTransportUart()
	testCon := NewTestConnector(t)
	trans.Connecter = testCon
	_, err := testCon.Connect("ignored")
	if err != nil {
		t.Error(err)
	}

	client.Transport = trans

	// Deliver expected network traffic async.
	asyncDoneChan := make(chan bool)
	go func() {
		t.Log("Server: wait for msg")
		msg, err := testCon.ServerReceive(3 * time.Second)
		if err != nil {
			t.Error(err)
		}
		// Client             Server
		// |                    |
		// |  GET /temperature  |
		// |    Token: 0x4a     |   Registration
		// |  Observe: 0        |
		// +------------------->|
		observeOptionVal := msg.Options().Get(coapmsg.Observe).AsUInt8()
		if observeOptionVal != 0 {
			t.Errorf("Expected observe option to be '0' but was %d", observeOptionVal)
		}

		// |                    |
		// |    2.05 Content    |
		// |    Token: 0x4a     |   Notification of
		// |  Observe: 12       |   the current state
		// |  Payload: 22.9 Cel |
		// |<-------------------+   Repeat N times

		// Send ack with initial
		ack := coapmsg.NewAck(msg.MessageID)
		ack.Type = coapmsg.Acknowledgement
		ack.Code = coapmsg.Content // For piggyback response.
		ack.Token = msg.Token
		ack.Payload = []byte("1")
		err = ack.Options().Add(coapmsg.Observe, 1)
		if err != nil {
			t.Error(err)
		}

		err = testCon.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		// Wait some time before sending second update
		time.Sleep(500 * time.Millisecond)

		// Send CON with updated data
		ack = coapmsg.NewAck(msg.MessageID)
		ack.Type = coapmsg.Confirmable
		ack.Code = coapmsg.Content // For piggyback response.
		ack.Token = msg.Token
		ack.Payload = []byte("2")
		err = ack.Options().Add(coapmsg.Observe, 2)
		if err != nil {
			t.Error(err)
		}

		err = testCon.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		// Wait for the ACK of the last CON message
		msg, err = testCon.ServerReceive(3 * time.Second)
		if err != nil {
			t.Error(err)
		}
		if msg.Type != coapmsg.Acknowledgement {
			t.Error("Expected ACK for con")
		}

		// Wait for Cancel observe
		msg, err = testCon.ServerReceive(3 * time.Second)
		if err != nil {
			t.Error(err)
		}
		if msg.Options().Get(coapmsg.Observe).AsUInt8() != 1 {
			t.Error("Expected cancel observe (=1) option")
		}
		ack = coapmsg.NewAck(msg.MessageID)
		ack.Token = msg.Token
		ack.Code = coapmsg.Content
		err = testCon.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		asyncDoneChan <- true
	}()

	t.Log("Client: start observe")
	res, err := client.Observe("coap+uart://any/o")

	if err != nil {
		t.Error(err)
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		t.Error(err)
	}

	if buf.String() != "1" {
		t.Errorf("Expected body '1' but got %s", buf.String())
	}

	t.Logf("Clicent: Got response 1: %s", buf.String())

	// Get the second response
	if res != nil {
		t.Log("Client: Waiting for response 2")
		select {
		case res = <-res.Next():
			buf.Reset()
			_, err := buf.ReadFrom(res.Body)
			if err != nil {
				t.Error(err)
			}
			if buf.String() != "2" {
				t.Errorf("Expected body '2' but got %s", buf.String())
			}
			t.Logf("Client: Got response 2: %s", buf.String())

			time.Sleep(200 * time.Millisecond)
			t.Log("Client: Canceling the observe")
			_, err = client.CancelObserve(res)
			if err != nil {
				t.Error("Error: " + err.Error())
			}
		case <-time.After(3 * time.Second):
			t.Error("Timeout while waiting for Next")
		}

	}

	<-asyncDoneChan

	if len(testCon.conn.interactions) != 0 {
		t.Errorf("Interactions not cleaned up! len: %d", len(testCon.conn.interactions))
	}

	ValidateCleanConnection(t, testCon)
}

func TestClientCancelObserve(t *testing.T) {

	client := NewClient()
	client.Timeout = 10 * time.Second
	trans := NewTransportUart()
	testCon := NewTestConnector(t)
	trans.Connecter = testCon
	_, err := testCon.Connect("ignored")
	if err != nil {
		t.Error(err)
	}

	client.Transport = trans

	// Deliver expected network traffic async.
	asyncDoneChan := make(chan bool)
	go func() {
		t.Log("Server: wait for msg")
		msg, err := testCon.ServerReceive(5 * time.Second)
		if err != nil {
			t.Error(err)
		}
		// Client             Server
		// |                    |
		// |  GET /temperature  |
		// |    Token: 0x4a     |   Registration
		// |  Observe: 0        |
		// +------------------->|
		observeOptionVal := msg.Options().Get(coapmsg.Observe).AsUInt8()
		if observeOptionVal != 0 {
			t.Errorf("Expected observe option to be '0' but was %d", observeOptionVal)
		}

		// |                    |
		// |    2.05 Content    |
		// |    Token: 0x4a     |   Notification of
		// |  Observe: 12       |   the current state
		// |  Payload: 22.9 Cel |
		// |<-------------------+   Repeat N times

		// Send ack with initial
		ack := coapmsg.NewAck(msg.MessageID)
		ack.Type = coapmsg.Acknowledgement
		ack.Code = coapmsg.Content // For piggyback response.
		ack.Token = msg.Token
		ack.Payload = []byte("1")
		err = ack.Options().Add(coapmsg.Observe, 1)
		if err != nil {
			t.Error(err)
		}

		err = testCon.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		// Wait for Cancel observe
		msg, err = testCon.ServerReceive(5 * time.Second)
		if err != nil {
			t.Error(err)
		}
		if msg.Options().Get(coapmsg.Observe).AsUInt8() != 1 {
			t.Error("Expected cancel observe (=1) option")
		}
		ack = coapmsg.NewAck(msg.MessageID)
		ack.Token = msg.Token
		ack.Code = coapmsg.Content
		err = testCon.ServerSend(ack)
		if err != nil {
			t.Error(err)
		}

		asyncDoneChan <- true
	}()

	t.Log("Client: start observe")
	res, err := client.Observe("coap+uart://any/o")

	if err != nil {
		t.Error(err)
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		t.Error(err)
	}

	if buf.String() != "1" {
		t.Errorf("Expected body '1' but got %s", buf.String())
	}

	//	<-time.After(4 * time.Second)
	_, err = client.CancelObserve(res)
	if err != nil {
		t.Error("Error: " + err.Error())
	}

	<-asyncDoneChan

	t.Log("Waiting for shutdown")

	ValidateCleanConnection(t, testCon)
}
