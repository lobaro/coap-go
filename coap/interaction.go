package coap

import (
	"bytes"
	"context"
	"errors"
	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/Sirupsen/logrus"
	"time"
)

type Token []byte
type MessageId uint16

// Interaction tracks one interaction between a CoAP client and Server.
// The Interaction is created with a request and ends with a response.
type Interaction struct {
	req        *Request
	token      Token
	MessageId  MessageId
	AckPending bool
	conn       Connection
	receiveCh  chan *coapmsg.Message

	NotificationCh chan *coapmsg.Message
}

type Interactions []*Interaction

func (ia *Interaction) HandleMessage(msg *coapmsg.Message) {
	// CON messages are already acknowledged by the connection (is that a good idea?)
	ia.receiveCh <- msg

}

func (ia *Interaction) readMessage(ctx context.Context) (*coapmsg.Message, error) {
	select {
	case msg := <-ia.receiveCh:
		return msg, nil
	case <-ctx.Done():
		return nil, errors.New("Context timed out during interaction.readMessage")
	}
}

func (ia *Interaction) RoundTrip(ctx context.Context, reqMsg *coapmsg.Message) (resMsg *coapmsg.Message, err error) {
	ia.MessageId = MessageId(reqMsg.MessageID)

	// send the request
	sendMessage(ia.conn, reqMsg)

	if reqMsg.Type == coapmsg.Confirmable {
		// Handle CON request

		// TODO: Implement retries for CON messages until first ACK is received or some timeout
		withAckTimeout, _ := context.WithTimeout(ctx, ackTimeout())
		resMsg, err = ia.readMessage(withAckTimeout)
		if err != nil {
			return nil, wrapError(err, "Failed to read ACK")
		}
		if err = validateMessageId(reqMsg, resMsg); err != nil {
			return nil, wrapError(err, "Failed to read ACK")
		}
		if resMsg.Type != coapmsg.Acknowledgement {
			return nil, errors.New("Expected ACK response but got " + reqMsg.Type.String())
		}

		// TODO: Handle Types: RST correctly - now we just don't care
		if resMsg.Type == coapmsg.Acknowledgement && resMsg.Code == coapmsg.Empty {
			// Handle postponed (non-piggyback) response

			//  Client              Server
			//    |                  |
			//    |   CON [0x7a10]   |
			//    | GET /temperature |
			//    |   (Token 0x73)   |
			//    +----------------->|
			//    |                  |
			//    |   ACK [0x7a10]   |
			//    |<-----------------+ <- We are here!
			//    |                  |
			//    ... Time Passes  ...
			//    |                  |
			//    |   CON [0x23bb]   |
			//    |   2.05 Content   |
			//    |   (Token 0x73)   |
			//    |     "22.5 C"     |
			//    |<-----------------+
			//    |                  |
			//    |   ACK [0x23bb]   |
			//    +----------------->|
			//    |                  |
			//
			// Figure 5: A GET Request with a Separate Response
			withTimeout, _ := context.WithTimeout(ctx, 10*time.Second)
			resMsg, err = ia.readMessage(withTimeout)
			if err != nil {
				return nil, wrapError(err, "Failed to read postponed response")
			}
			// The messageId from resMsg needs to be confirmed
			if resMsg.Type != coapmsg.Confirmable && resMsg.Type != coapmsg.NonConfirmable {
				return nil, errors.New("Expected postponed response but got " + reqMsg.Type.String())
			}

			// TODO: Send outgoing ACKs on connection level?
			if resMsg.Type == coapmsg.Confirmable {
				ack := coapmsg.NewAck(resMsg.MessageID)
				if err := sendMessage(ia.conn, &ack); err != nil {
					return nil, err
				}
			}
		} else if resMsg.Type == coapmsg.Acknowledgement && resMsg.Code != coapmsg.Empty {
			// Handle piggyback response

			// here is no need for
			// separately acknowledging a piggybacked response, as the client will
			// retransmit the request if the Acknowledgement message carrying the
			// piggybacked response is lost.

		} else {
			return nil, errors.New("Received invalid reponse from server")
		}
	} else if reqMsg.Type == coapmsg.NonConfirmable {
		// Handle NON request
		withAckTimeout, _ := context.WithTimeout(ctx, ackTimeout())
		resMsg, err := ia.readMessage(withAckTimeout)
		if err != nil {
			return nil, wrapError(err, "Failed to read NON response")
		}
		if err = validateMessageId(reqMsg, resMsg); err != nil {
			return nil, wrapError(err, "Failed to read NON response")
		}
		if resMsg.Type != coapmsg.NonConfirmable {
			return nil, errors.New("Expected NON response but got " + reqMsg.Type.String())
		}

	} else {
		msgLogEntry(reqMsg).Panic("Invalid request message type from client. Expected CON or NON")
	}

	// Handle observe
	if reqMsg.Option(coapmsg.Observe) == 0 {
		go ia.waitForNotify(ctx)
	}

	if err = validateToken(reqMsg, resMsg); err != nil {
		return nil, err
	}
	return resMsg, nil

}

func (ia *Interaction) waitForNotify(ctx context.Context) {
	ia.NotificationCh = make(chan *coapmsg.Message, 1)
	defer close(ia.NotificationCh)

	for {
		resMsg, err := ia.readMessage(ctx)
		if err != nil {
			logrus.WithError(err).Error("Failed to read notify response")
			return
		}
		if resMsg.Type == coapmsg.Confirmable {
			ack := coapmsg.NewAck(resMsg.MessageID)
			if err := sendMessage(ia.conn, &ack); err != nil {
				logrus.WithError(err).Error("Failed to send ACK for notify")
				return
			}
		}

		// TODO: How does the server tell us to stop listening?

		select {
		case ia.NotificationCh <- resMsg:
		case <-ctx.Done():
			logrus.Info("Stopped observer, request context timed out!")
			return
		}

		select {
		case <-ctx.Done():
			logrus.Info("Stopped observer, request context timed out!")
			return
		default:
			continue
		}
	}

	return
}

func validateMessageId(req, res *coapmsg.Message) error {
	if req.MessageID != res.MessageID {
		// This should never happen
		err := errors.New("coap: CRITICAL - MessageId of response does not match")
		logrus.WithError(err).
			WithField("ReqMessageId", req.MessageID).
			WithField("ResMessageId", res.MessageID).
			WithField("ReqToken", req.Token).
			WithField("ResToken", res.Token).
			Error("An interaction must never be called with the wrong message id")
		return err
	}
	return nil
}

func validateToken(req, res *coapmsg.Message) error {
	if !bytes.Equal(req.Token, res.Token) {
		// This should never happen
		err := errors.New("coap: CRITICAL - Token of response does not match")
		logrus.WithError(err).
			WithField("ReqMessageId", req.MessageID).
			WithField("ResMessageId", res.MessageID).
			WithField("ReqToken", req.Token).
			WithField("ResToken", res.Token).
			Error("An interaction must never be called with the wrong token")
		return err
	}
	return nil
}

func (t Token) Equals(other Token) bool {
	return bytes.Equal(t, other)
}
