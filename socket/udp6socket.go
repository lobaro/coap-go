package sckt

import (
	"net"
	"golang.org/x/net/ipv6"
	"strconv"
	"fmt"
	"log"
)

type udp6socket struct {
	id        int
	onRx      chan <-*Datagram

	netifID   int
	port      int
	netIf     *net.Interface
	pktCon    *ipv6.PacketConn

	localaddr net.Addr
}

func (sckt *udp6socket) Write(data []byte, Dest net.Addr) (int, error) {
	fmt.Printf("Writing via udp6 to %s\r\n", Dest.String())
	return sckt.pktCon.WriteTo(data, nil, Dest)
}

func (sckt *udp6socket)Close() error {
	return sckt.Close()
}

func (sckt *udp6socket)SocketID() int {
	return sckt.id
}

func (sckt *udp6socket)ReceiveCh(newChan chan <-*Datagram) chan <-*Datagram {
	if newChan != nil {
		sckt.onRx = newChan
	}
	return sckt.onRx
}

func (sckt *udp6socket) Network() string {
	return sckt.localaddr.Network()
}

func (sckt *udp6socket)String() string {
	return sckt.localaddr.String()
}

func (sckt *udp6socket) LocalAddr() net.Addr {
	return sckt.localaddr
}

func (sckt *udp6socket)AsyncListenAndServe() {

	go func() error {
		defer sckt.pktCon.Close()
		b := make([]byte, 1500)

		fmt.Println("opened udp6 socket on Port " + strconv.Itoa(sckt.port))

		for {

			n, _, adr, err := sckt.pktCon.ReadFrom(b)

			if (err != nil) {
				log.Fatal(err)
				return err
			}

			if sckt.onRx != nil {
				bCpy := make([]byte, n)
				copy(bCpy, b[:n])

				dat := Datagram{}

				dat.Origin = adr
				dat.Data = bCpy
				dat.Socket = sckt

				sckt.onRx <- &dat //send away the packet
			}
		}
	}()

}

func NewUdp6Socket(SocketID, NetInterfaceIdx, Port int, chRx chan <- *Datagram) (Socket, error) {
	var err error

	udp6 := new(udp6socket)

	udp6.id = SocketID

	udp6.netifID = NetInterfaceIdx
	udp6.port = Port
	udp6.onRx = chRx

	udp6.netIf, err = net.InterfaceByIndex(NetInterfaceIdx)

	if err != nil {
		return nil, err
	}

	c, err := net.ListenPacket("udp6", "[::]:" + strconv.Itoa(Port)) //unspecified adr @port (Coap Default Port 5683)
	if (err != nil) {
		return nil, err
	}

	udp6.localaddr = c.LocalAddr()

	p := ipv6.NewPacketConn(c)
	err = p.JoinGroup(udp6.netIf, &net.UDPAddr{IP: net.ParseIP("ff02::1")}) //join multicast group on connection	
	if (err != nil) {
		return nil, err
	}

	udp6.pktCon = p

	return udp6, nil
}
