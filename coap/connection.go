package coap

import (
	"bytes"
	"context"
	"errors"
	"io"

	"time"

	"github.com/lobaro/coap-go/coapmsg"
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

	Name() string
	//resetDeadline()
}

type InteractionStore interface {
	FindInteraction(token Token, msgId MessageId) *Interaction
	StartInteraction(conn Connection, msg *coapmsg.Message) *Interaction
	RemoveInteraction(ia *Interaction)
	InteractionCount() int
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
	start := time.Now()
	for {
		//log.Info("Receive loop")
		if ctx.Err() != nil {
			log.WithError(ctx.Err()).Debug("Context done while handling message. Stopped receive loop.")
			return
		}
		duration := time.Since(start)
		if duration > 100*time.Millisecond {
			log.WithField("duration", duration).Warn("Read took longer than 100ms")
		}
		msg, err := readMessage(ctx, conn)

		if ctx.Err() != nil {
			log.WithError(ctx.Err()).Debug("Context done while read message. Stopped receive loop.")
			return
		}

		if err == io.EOF {
			time.Sleep(100 * time.Millisecond)
			start = time.Now()
			continue
		}

		if err != nil {
			// This is not a warning, since it happens on every reconnect for blocking connections
			log.WithError(err).Debug("Failed to receive message in receive loop")

			// We return on error, a reconnect has to restart the receive loop as well
			return
		}
		start = time.Now()

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
			handleStart := time.Now()
			ia.HandleMessage(msg)
			duration = time.Since(handleStart)
			if duration > 100*time.Millisecond {
				log.WithField("duration", duration).Warn("Handle Message took longer than 100ms")
			}
		}

	}
}

func readMessage(ctx context.Context, reader PacketReader) (*coapmsg.Message, error) {
	var packet []byte
	var err error
	// Skip empty packets
	for ; len(packet) == 0; packet, err = readPacket(ctx, reader) {
		if err != nil {
			return nil, err
		}
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
