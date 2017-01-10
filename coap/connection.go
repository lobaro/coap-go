package coap

import (
	"bytes"
	"context"
	"errors"
	"github.com/Lobaro/coap-go/coapmsg"
)

// Connection represents an interface to identify
// a CoAP connection which might be reused between requests.
// e.g. for Observe we have to listen for relevant updates.
type Connection interface {
	CoapPacketReader
	CoapPacketWriter

	// Starts a loop that reads packets and forwards them to interactions
	StartReceiveLoop(ctx context.Context)
	FindInteraction(token Token, msgId MessageId) (*Interaction, error)
	AddInteraction(ia *Interaction)

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

// CoapConnection hold multiple interaction and provides
// capabilities to send and receive CoAP messages over the wire
type CoapConnection struct {
}

func deleteConnection(a []Connection, i int) []Connection {
	copy(a[i:], a[i+1:])
	a[len(a)-1] = nil // or the zero value of T
	a = a[:len(a)-1]
	return a
}

func sendMessage(conn Connection, msg *coapmsg.Message) error {
	bin, err := msg.MarshalBinary()
	if err != nil {
		return err
	}

	logMsg(msg, "Send")
	err = conn.WritePacket(bin)
	if err != nil {
		return err
	}
	return nil
}

func readMessage(ctx context.Context, conn Connection) (*coapmsg.Message, error) {
	packetCh := make(chan []byte)
	errorCh := make(chan error)
	var packet []byte
	go func() {
		packet, err := readPacket(conn)
		if err != nil {
			errorCh <- err
		} else {
			packetCh <- packet
		}
	}()

	select {
	case err := <-errorCh:
		return nil, err
	case packet = <-packetCh:
		break
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	msg, err := coapmsg.ParseMessage(packet)
	if err != nil {
		return nil, wrapError(err, "Failed to parse CoAP message")
	}
	logMsg(&msg, "Received")

	return &msg, nil
}

func readPacket(reader CoapPacketReader) ([]byte, error) {
	buf := &bytes.Buffer{}

	var isPrefix bool

	for {
		var p []byte
		p, isPrefix, err := reader.ReadPacket()
		buf.Write(p)

		if err != nil {
			return nil, err
		}

		if !isPrefix {
			break
		}
	}

	if isPrefix {
		return nil, errors.New("coap: Did read incomplete response")
	}

	return buf.Bytes(), nil
}
