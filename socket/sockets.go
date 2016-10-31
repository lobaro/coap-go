package sckt

import (
	"net"
)

type Datagram struct {
	Origin net.Addr
	Data   []byte
	Socket Socket
}

type Socket interface {
	Write(data []byte, Dest net.Addr) (int, error)
	Close() error
	SocketID() int
	ReceiveCh(chan <-*Datagram) chan <-*Datagram
	AsyncListenAndServe()
	LocalAddr() net.Addr
}
