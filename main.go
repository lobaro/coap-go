package main

import (
	"log"
	"net"
	"time"

	coap "github.com/lobaro/coap-go/coap-old"
)

func main() {
	log.Println("Init liblobarocoap")

	log.Println("Creating UDP CoAP socket")

	addr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 5683,
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Fatal("Failed to create UDP listener", err)
	}
	defer conn.Close()

	coap.CreateSocket(1)

	go handleUdpOutgoing(conn)
	handleUdpIncoming(conn)
}

func handleUdpOutgoing(conn *net.UDPConn) {
	for {
		response := <-coap.PendingResponses

		log.Println("Sending UDP Data to ", response.Address, " => ", response.Message)
		conn.WriteTo(response.Message, response.Address)
	}
}

func handleUdpIncoming(conn *net.UDPConn) {
	for {
		buff := make([]byte, 1024)
		n, addr, err := conn.ReadFromUDP(buff)

		if err != nil {
			log.Println("Failed to read message from UDP socket", err)
		}

		msg := buff[:n]

		log.Println("Received UDP Data@", addr.IP, ":", addr.Port, "=>", msg)

		coap.HandleReceivedUdp4Message(1, addr, msg)
		time.Sleep(time.Millisecond * 100)
	}
}
