package main

import (
	"testing"
	"gitlab.com/lobaro/go-c-example/coap"
)

func TestSetupSocket(t *testing.T) {
	socket := coap.CreateSocket(42)
	
	if socket.IfID != 42 {
		t.Error("interface id must be 42 but is ", socket.IfID)
	}
}


func TestSendPingReceivePont(t *testing.T) {
	socket := coap.CreateSocket(42)

	if socket.IfID != 42 {
		t.Error("interface id must be 42 but is ", socket.IfID)
	}
}
