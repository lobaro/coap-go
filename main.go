package main

import (
	"log"
	//"gitlab.com/lobaro/go-c-example/cgo/sub"
	"gitlab.com/lobaro/go-c-example/cgo"
	_ "gitlab.com/lobaro/go-c-example/coap"
	"gitlab.com/lobaro/go-c-example/coap"
)




func main() {
	log.Println("Hello Lobaro ", cgo.Test())
	log.Println("Creating CoAP socket")
	coap.CreateSocket(1)
	//log.Println("Hello Lobaro ", sub.Bar())

	coap.FakeReceivedAckPacketFrom(1)
}
