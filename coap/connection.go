package coap

import (
	"bytes"
	"context"
	"errors"
	"io"

	"time"

	"github.com/Lobaro/coap-go/coapmsg"
)

// Connection represents an interface to identify
// a CoAP connection which might be reused between requests.
// e.g. for Observe we have to listen for relevant updates.
type Connection interface {
	PacketReader
	PacketWriter

	// Starts a loop that reads packets and forwards them to interactions
	FindInteraction(token Token, msgId MessageId) *Interaction
	AddInteraction(ia *Interaction)
	RemoveInteraction(ia *Interaction)

	Open() error
	Close() error
	Closed() bool

	resetDeadline()
}

// Implemented by connections
type PacketReader interface {
	ReadPacket() (p []byte, isPrefix bool, err error)
}

// Implemented by connections
type PacketWriter interface {
	WritePacket(p []byte) (err error)
}

func deleteConnection(a []Connection, i int) []Connection {
	copy(a[i:], a[i+1:])
	a[len(a)-1] = nil // or the zero value of T
	a = a[:len(a)-1]
	return a
}

func sendMessage(conn Connection, msg *coapmsg.Message) error {
	bin := msg.MustMarshalBinary()

	logMsg(msg, "Send")
	err := conn.WritePacket(bin)
	if err != nil {
		return err
	}
	return nil
}

func readMessage(ctx context.Context, conn Connection) (*coapmsg.Message, error) {
	packet, err := readPacket(ctx, conn)

	if err != nil {
		return nil, err
	}

	msg, err := coapmsg.ParseMessage(packet)
	if err != nil {
		return nil, wrapError(err, "Failed to parse CoAP message")
	}
	logMsg(&msg, "Received")

	return &msg, nil
}

func readPacket(ctx context.Context, reader PacketReader) ([]byte, error) {
	buf := &bytes.Buffer{}

	var isPrefix bool

	for {
		var p []byte
		p, isPrefix, err := reader.ReadPacket()
		buf.Write(p)

		if err != nil && err != io.EOF {
			return nil, err
		}

		if !isPrefix {
			break
		}

		select {
		case <-ctx.Done():
			return nil, errors.New("coap: Timeout while readPacket")
		default:
		}

		time.Sleep(10 * time.Millisecond)
	}

	if isPrefix {
		return nil, errors.New("coap: Did read incomplete response")
	}

	return buf.Bytes(), nil
}
