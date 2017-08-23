package main

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/Lobaro/coap-go/socket"
)

func main() {

	argcnt := len(os.Args)

	if argcnt != 2 {
		fmt.Println("usage: server.exe <network interface index> (hint: \"route print\")\r\n")
		displayNetInterfaces()
		return
	}

	ifID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println(err)
		fmt.Println("usage: server.exe <network interface index> (hint: \"route print\")\r\n")
		displayNetInterfaces()
		return
	}

	DataCh := make(chan *sckt.Datagram, 10)

	//UDP Socket
	socket, err := sckt.NewUdp6Socket(99, ifID, 5683, DataCh) //on incoming data channel receives the datagram
	if err != nil {
		fmt.Println(err)
		fmt.Println("usage: server.exe <network interface index> (hint: \"route print\")\r\n")
		displayNetInterfaces()
		return
	}
	socket.AsyncListenAndServe()

	//Web Socket
	WSsocket, err := sckt.NewWSSocket(100, "/wsms", 8081, DataCh)
	if err != nil {
		fmt.Println(err)
		fmt.Println("usage: server.exe <network interface index> (hint: \"route print\")\r\n")
		displayNetInterfaces()
		return
	}
	WSsocket.AsyncListenAndServe()

	//Processing loop
	for {
		select {
		case d := <-DataCh:
			fmt.Printf("Got %d Bytes on SocketID #%d LocalEndpoint: %s\r\n", len(d.Data), d.Socket.SocketID(), d.Socket.LocalAddr().String())
			go func(pkt *sckt.Datagram) {
				//call "NewPacketHandler(...)"

			}(d)
		}
	}

}

func displayNetInterfaces() {
	sysNetInterfaces, _ := net.Interfaces()
	fmt.Println("Network Interfaces on this system:")
	for _, v := range sysNetInterfaces {
		fmt.Printf("Index=%d Name=%s Mac=%s:\r\n", v.Index, v.Name, v.HardwareAddr.String())
		addrs, _ := v.Addrs()

		for _, v2 := range addrs {
			fmt.Println(v2.Network() + " " + v2.String())
		}
		fmt.Printf("\r\n")
	}

}
