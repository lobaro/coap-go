package coap

// Connection represents an interface to identify
// a CoAP connection which might be reused between requests.
// e.g. for Observe we have to listen for relevant updates.
type Connection interface {
	CoapPacketReader
	CoapPacketWriter
	Close() error
	Closed() bool

	resetDeadline()
}

// Implemented by connections
type CoapPacketReader interface {
	ReadPacket() (p []byte, isPrefix bool, err error)
}

// Implemented by connections
type CoapPacketWriter interface {
	WritePacket(p []byte) (err error)
}

func deleteConnection(a []Connection, i int) []Connection {
	copy(a[i:], a[i+1:])
	a[len(a)-1] = nil // or the zero value of T
	a = a[:len(a)-1]
	return a
}
