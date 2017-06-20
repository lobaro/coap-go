package main

import (
		"github.com/trusch/coap-go/socket"
		"fmt"
		"os"
		"strconv"
		"net"
	"gitlab.com/lobaro/lobaro-coap-go/socket"
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

	DataCh := make(chan *(sckt.Packet), 10)
	
	//UDP Socket
	socket, err := sckt.NewUdp6Socket(99,ifID, 5683, DataCh) //on incoming data channel receives the datagram
	if err != nil {
		fmt.Println(err)
		fmt.Println("usage: server.exe <network interface index> (hint: \"route print\")\r\n")
		displayNetInterfaces()
		return
	}
	socket.AsyncListenAndServe() 
	
	//Web Socket
	WSsocket, err := sckt.NewWSSocket(100,"/wsms",8081, DataCh)
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
			case Packet:= <-DataCh:
				
				fmt.Printf("Got %d Bytes on SocketID #%d LocalEndpoint: %s\r\n", len(Packet.Data), Packet.Socket.SocketID(), Packet.Socket.LocalAddr().String())
				go func(pkt *sckt.Packet) {	
				    //call "NewPacketHandler(...)"

				}(Packet)
		}
	}
	
	
	
}


func displayNetInterfaces() {
	sysNetInterfaces, _ := net.Interfaces()
	fmt.Println("Network Interfaces on this system:")
	for _,v := range(sysNetInterfaces)  {
		fmt.Printf("Index=%d Name=%s Mac=%s:\r\n", v.Index, v.Name, v.HardwareAddr.String())
		addrs,_ := v.Addrs()
		
		for _,v2 := range(addrs) {
			fmt.Println(v2.Network()+" " + v2.String())
		}
		fmt.Printf("\r\n")
	}
	
}
