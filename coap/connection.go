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
	InteractionStore

	// Starts a loop that reads packets and forwards them to interactions
	Open() error
	Close() error
	Closed() bool

	//resetDeadline()
}

type InteractionStore interface {
	FindInteraction(token Token, msgId MessageId) *Interaction
	StartInteraction(conn Connection, msg *coapmsg.Message) *Interaction
	RemoveInteraction(ia *Interaction)
}

// Implemented by connections
type PacketReader interface {
	ReadPacket() (p []byte, isPrefix bool, err error)
}

// Implemented by connections
type PacketWriter interface {
	WritePacket(p []byte) (err error)
}

type incomingPacketHandler struct {
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

func receiveLoop(ctx context.Context, conn Connection) {
	for {
		//log.Info("Receive loop")
		if ctx.Err() != nil {
			log.WithError(ctx.Err()).Info("Context done. Stopped receive loop.")
			return
		}
		msg, err := readMessage(ctx, conn)

		if err != nil {
			// Do not close the connection, this might happen during reopening of the serial port
			// TODO: An "Access is denied." error indicates that the UARt is not reachable. So reopening would be an option
			// We could always stop the receive loop before reopening and close the connection here
			// This is not a warning, since it happens on every reconnect for blocking connections
			log.WithError(err).Info("Failed to receive message")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		ia := conn.FindInteraction(Token(msg.Token), MessageId(msg.MessageID))
		if ia == nil {
			log.WithError(err).
				WithField("token", msg.Token).
				WithField("messageId", msg.MessageID).
				Warn("Failed to find interaction, send RST and drop packet")

			// Even non-confirmable messages can be answered with a RST
			rst := coapmsg.NewRst(msg.MessageID)
			if err := sendMessage(conn, &rst); err != nil {
				log.WithError(err).Warn("Failed to send RST")
			}
		} else {
			ia.HandleMessage(msg)
		}

	}
}

func readMessage(ctx context.Context, reader PacketReader) (*coapmsg.Message, error) {
	var packet []byte
	var err error
	// Skip empty packets
	for ; len(packet) == 0; packet, err = readPacket(ctx, reader) {

	}

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
