package main

import (
	"testing"
	"gitlab.com/lobaro/go-c-example/coap"
	"gitlab.com/lobaro/go-c-example/coapmsg"
)

func TestSetupSocket(t *testing.T) {
	socket := coap.CreateSocket(42)

	if socket.IfID != 42 {
		t.Error("interface id must be 42 but is ", socket.IfID)
	}
}


func TestSendPingReceivePong(t *testing.T) {
	coap.CreateSocket(22)

	var sendPacket coapmsg.Message
	coap.SendMessageHandler = func (ifId uint8, message coapmsg.Message) {
		if ifId != 22 {
			t.Error("Expected ifId in SendMessageHandler to be 22 but is", ifId)
		}
		sendPacket = message
	}
	
	pingMsg := coapmsg.Message{
		Type: coapmsg.Confirmable,
		Code: coapmsg.COAPCode(0),
	}
	
	coap.HandleReceivedMessage(22, pingMsg)
	
	if len(sendPacket.Payload) != 0 {
		t.Error("Pong payload must be empty but has len", len(sendPacket.Payload))
	}
	if sendPacket.Code != 0 {
		t.Error("Expected code 0 but got", sendPacket.Code)
	}
	if sendPacket.Type != coapmsg.Reset {
		t.Error("Expected type coapmsg.Reset but got", sendPacket.Type)
	}
}
