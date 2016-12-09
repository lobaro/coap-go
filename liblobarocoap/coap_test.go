package liblobarocoap

import (
	"github.com/Lobaro/coap-go/coapmsg"
	"testing"
	"time"
)

func TestInit(t *testing.T) {

}

func TestSendPingReceivePong(t *testing.T) {
	socket := NewSocket()

	pingMsg := coapmsg.Message{
		Type: coapmsg.Confirmable,
		Code: coapmsg.COAPCode(0),
	}

	msgBytes, err := pingMsg.MarshalBinary()
	if err != nil {
		t.Error("Failed to marshal CoAP message")
	}

	HandleIncomingUartPacket(socket, 10, msgBytes)

	response := <-PendingResponses

	if response.Socket.Handle != socket.Handle {
		t.Error("Expected socket handle in SendMessageHandler to be", socket.Handle, "but is", response.Socket.Handle)
	}

	pongMsg, err := coapmsg.ParseMessage(response.Data)
	if err != nil {
		t.Error("Failed to parse CoAP message", err)
	}

	if len(pongMsg.Payload) != 0 {
		t.Error("Pong payload must be empty but has len", len(pongMsg.Payload))
	}
	if pongMsg.Code != 0 {
		t.Error("Expected code 0 but got", pongMsg.Code)
	}
	if pongMsg.Type != coapmsg.Reset {
		t.Error("Expected type coapmsg.Reset but got", pongMsg.Type)
	}
}

func TestHandle_QueryWellKnown(t *testing.T) {
	socket := NewSocket()
	resource := CreateResource("/existing", "Some existing endpoint")
	if resource == nil {
		t.Error("Resource is nil")
	}
	getMsg := coapmsg.Message{
		Type:      coapmsg.Confirmable,
		Code:      coapmsg.GET,
		MessageID: 1,
		Payload:   []byte("Hello World"),
	}

	getMsg.SetPathString("/.well-known/core")

	msgBytes, err := getMsg.MarshalBinary()
	if err != nil {
		t.Error("Failed to marshal CoAP message")
	}

	HandleIncomingUartPacket(socket, 11, msgBytes)

	// Get ACK
	select {
	case ack := <-PendingResponses:
		ackMsg, err := coapmsg.ParseMessage(ack.Data)
		if err != nil {
			t.Error("Failed to parse CoAP message", err)
		}

		if ackMsg.Type != coapmsg.Acknowledgement {
			t.Error("Expected message type to be ack but was", ackMsg.Type)
		}
		if ackMsg.Code != coapmsg.Content {
			t.Error("Expected message code to be Content but was", ackMsg.Code.String())
		}
		expectedPayload := "<.well-known/core/>,<existing/>;title=\"Some existing endpoint\";cf=0,"
		if string(ackMsg.Payload) != expectedPayload {
			t.Error("Expected message payload to be", expectedPayload, "but was", string(ackMsg.Payload))
		}

		if ackMsg.MessageID != uint16(1) {
			t.Error("Expected message id to be 1 but was", ackMsg.MessageID)
		}
	case <-time.After(1 * time.Second):
		t.Error("No response")
	}

	// Just wait for remaining C log output :)
	<-time.After(10 * time.Millisecond)
}

func TestHandle_QueryCustomNonPiggyResource(t *testing.T) {
	socket := NewSocket()
	resource := CreateResource("/my-resource", "My Test Resource", coapmsg.GET)

	resource.Handler = func(req coapmsg.Message, res *coapmsg.Message) HandlerResult {
		res.Payload = []byte("Lobaro!")
		res.Code = coapmsg.Content
		return POSTPONE
	}

	if resource == nil {
		t.Error("Resource is nil")
	}
	getMsg := coapmsg.Message{
		Type:      coapmsg.Confirmable,
		Code:      coapmsg.GET,
		MessageID: 1,
		Payload:   []byte("whoami?"),
	}

	getMsg.SetPathString("/my-resource")

	msgBytes, err := getMsg.MarshalBinary()
	if err != nil {
		t.Error("Failed to marshal CoAP message")
	}

	HandleIncomingUartPacket(socket, 11, msgBytes)

	// Get ACK
	select {
	case ack := <-PendingResponses:
		ackMsg, err := coapmsg.ParseMessage(ack.Data)
		if err != nil {
			t.Error("Failed to parse CoAP message", err)
		}

		if ackMsg.Type != coapmsg.Acknowledgement {
			t.Error("Expected message type to be ack but was", ackMsg.Type)
		}
		if ackMsg.Code != coapmsg.Content {
			t.Error("Expected message code to be Content but was", ackMsg.Code.String())
		}
		expectedPayload := ""
		if string(ackMsg.Payload) != expectedPayload {
			t.Error("Expected message payload to be", expectedPayload, "but was", string(ackMsg.Payload))
		}

		if ackMsg.MessageID != uint16(1) {
			t.Error("Expected message id to be 1 but was", ackMsg.MessageID)
		}
	case <-time.After(5 * time.Second):
		t.Error("No response")
	}

	select {
	case ack := <-PendingResponses:
		ackMsg, err := coapmsg.ParseMessage(ack.Data)
		if err != nil {
			t.Error("Failed to parse CoAP message", err)
		}

		if ackMsg.Type != coapmsg.Acknowledgement {
			t.Error("Expected message type to be ack but was", ackMsg.Type)
		}
		if ackMsg.Code != coapmsg.Content {
			t.Error("Expected message code to be Content but was", ackMsg.Code.String())
		}
		expectedPayload := "Lobaro!"
		if string(ackMsg.Payload) != expectedPayload {
			t.Error("Expected message payload to be", expectedPayload, "but was", string(ackMsg.Payload))
		}

		if ackMsg.MessageID != uint16(1) {
			t.Error("Expected message id to be 1 but was", ackMsg.MessageID)
		}
	case <-time.After(5 * time.Second):
		t.Error("No response")
	}

	// Just wait for remaining C log output :)
	<-time.After(10 * time.Millisecond)
}
