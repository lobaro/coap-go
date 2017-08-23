package main

import (
	"net"
	"testing"

	coap "github.com/Lobaro/coap-go/coap-old"
	"github.com/Lobaro/coap-go/coapmsg"
)

func TestSetupSocket(t *testing.T) {

	socket := coap.CreateSocket(42)

	if socket.IfID != 42 {
		t.Error("interface id must be 42 but is ", socket.IfID)
	}
}

func TestSendPingReceivePong(t *testing.T) {
	socketId := uint8(3)
	coap.CreateSocket(socketId)

	pingMsg := coapmsg.Message{
		Type: coapmsg.Confirmable,
		Code: coapmsg.COAPCode(0),
	}

	msgBytes, err := pingMsg.MarshalBinary()
	if err != nil {
		t.Error("Failed to marshal CoAP message")
	}

	coap.HandleReceivedMessage(socketId, msgBytes)

	response := <-coap.PendingResponses

	if response.IfId != socketId {
		t.Error("Expected ifId in SendMessageHandler to be 22 but is", response.IfId)
	}

	pongMsg, err := coapmsg.ParseMessage(response.Message)
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

func TestHandle_Get_NotFound_Request(t *testing.T) {
	socketId := uint8(4)
	coap.CreateSocket(socketId)

	getMsg := coapmsg.Message{
		Type:      coapmsg.Confirmable,
		Code:      coapmsg.GET,
		MessageID: 1,
		Payload:   []byte("Hello World"),
	}
	// TODO: c coap parser does not work without options?
	getMsg.Options().Set(coapmsg.URIPath, "/test")

	msgBytes, err := getMsg.MarshalBinary()
	if err != nil {
		t.Error("Failed to marshal CoAP message")
	}

	coap.HandleReceivedMessage(socketId, msgBytes)

	ack := <-coap.PendingResponses

	ackMsg, err := coapmsg.ParseMessage(ack.Message)
	if err != nil {
		t.Error("Failed to parse CoAP message", err)
	}

	if ackMsg.Type != coapmsg.Acknowledgement {

		t.Fatal("Expected message type to be ack but was", ackMsg.Type)
	}
	if ackMsg.Code != coapmsg.NotFound {
		t.Fatal("Expected message code to be NotFound but was", int(ackMsg.Code))
	}

	if ackMsg.MessageID != uint16(1) {
		t.Fatal("Expected message id to be 1 but was", ackMsg.MessageID)
	}
}

func Fail_TestHandle_Confirmable_Get_Found_Request(t *testing.T) {
	socketId := uint8(5)
	coap.CreateSocket(socketId)
	coap.CreateResource("existing", "Some existing endpoint")

	getMsg := coapmsg.Message{
		Type:      coapmsg.Confirmable,
		Code:      coapmsg.GET,
		MessageID: 1,
		Payload:   []byte("Hello World"),
	}
	// TODO: c coap parser does not work without options?
	getMsg.Options().Set(coapmsg.URIPath, "/.well-known/core")

	msgBytes, err := getMsg.MarshalBinary()
	if err != nil {
		t.Error("Failed to marshal CoAP message")
	}

	udpAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 5684,
	}
	coap.HandleReceivedUdp4Message(socketId, udpAddr, msgBytes)

	ack := <-coap.PendingResponses

	ackMsg, err := coapmsg.ParseMessage(ack.Message)
	if err != nil {
		t.Error("Failed to parse CoAP message", err)
	}

	if ackMsg.Type != coapmsg.Acknowledgement {

		t.Fatal("Expected message type to be ack but was", ackMsg.Type)
	}
	if ackMsg.Code != coapmsg.NotFound {
		t.Fatal("Expected message code to be NotFound but was", int(ackMsg.Code))
	}

	if ackMsg.MessageID != uint16(1) {
		t.Fatal("Expected message id to be 1 but was", ackMsg.MessageID)
	}
}
