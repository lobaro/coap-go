package coap

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/Lobaro/coap-go/coapmsg"
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

	// CancelObserve will stop the interaction to listen for Notifications
	StopListenForNotifications context.CancelFunc
	NotificationCh             chan *coapmsg.Message
}

type Interactions []*Interaction

func (ia *Interaction) HandleMessage(msg *coapmsg.Message) {
	ia.receiveCh <- msg
}

var READ_MESSAGE_CTX_DONE = errors.New("Read timeout")

func (ia *Interaction) readMessage(ctx context.Context) (*coapmsg.Message, error) {
	select {
	case msg := <-ia.receiveCh:
		return msg, nil
	case <-ctx.Done():
		return nil, READ_MESSAGE_CTX_DONE
	}
}

var ERROR_READ_ACK = "Failed to read ACK"

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
			return nil, wrapError(err, ERROR_READ_ACK)
		}
		if err = validateMessageId(reqMsg, resMsg); err != nil {
			return nil, wrapError(err, ERROR_READ_ACK)
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
			withTimeout, _ := context.WithTimeout(ctx, POSTPONED_RESPONSE_TIMEOUT)
			resMsg, err = ia.readMessage(withTimeout)
			if err != nil {
				return nil, wrapError(err, "Failed to read postponed response")
			}
			// The messageId from resMsg needs to be confirmed
			if resMsg.Type != coapmsg.Confirmable && resMsg.Type != coapmsg.NonConfirmable {
				return nil, errors.New("Expected postponed response [CON or NON] but got " + resMsg.Type.String())
			}
			// TODO: Handle resMsg.Type != coapmsg.Reset - but how? Just okay to return an error?

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
	ia.NotificationCh = make(chan *coapmsg.Message, 0)

	// An observe request must set the observe option to 0
	// the server has to response with the observe option set
	if reqMsg.Options().Get(coapmsg.Observe).AsUInt8() == 0 && resMsg.Options().Get(coapmsg.Observe).IsSet() {
		go ia.waitForNotify(ctx)
	} else {
		close(ia.NotificationCh)
	}

	if err = validateToken(reqMsg, resMsg); err != nil {
		return nil, err
	}
	return resMsg, nil

}

func (ia *Interaction) waitForNotify(ctx context.Context) {
	defer close(ia.NotificationCh)
	withCancel, cancel := context.WithCancel(ctx)
	ia.StopListenForNotifications = cancel

	for {
		resMsg, err := ia.readMessage(withCancel)
		if err != nil {
			if err != READ_MESSAGE_CTX_DONE {
				log.WithError(err).Error("Stopped observer unexpected")
			} else {
				log.Info("Stopped observer")
			}
			return
		}

		select {
		case ia.NotificationCh <- resMsg:
			// TODO: Should we really only send the ACK when the notification is handled?
			// As it is now, the user might miss a few notifications but can
			// than still attach to the Next channel in the response
			if resMsg.Type == coapmsg.Confirmable {
				ack := coapmsg.NewAck(resMsg.MessageID)
				if err := sendMessage(ia.conn, &ack); err != nil {
					log.WithError(err).Error("Failed to send ACK for notify")
					return
				}
			}
		case <-ctx.Done():
			log.Info("Stopped observer, request context timed out! Send RST.")
			// Even non-confirmable messages can be answered with a RST
			rst := coapmsg.NewRst(resMsg.MessageID)
			if err := sendMessage(ia.conn, &rst); err != nil {
				log.WithError(err).Error("Failed to send RST for notify (1)")
				return
			}
			return
		case <-time.After(5 * time.Second): // default: Give the app some time to register an handler before send RST
			log.WithField("Token", ia.token).Warn("No application handler for notification registered. Send RST.")
			// Even non-confirmable messages can be answered with a RST
			rst := coapmsg.NewRst(resMsg.MessageID)
			if err := sendMessage(ia.conn, &rst); err != nil {
				log.WithError(err).Error("Failed to send RST for notify (2)")
				return
			}

		}

		// An error response MUST lead to a removal of the observer on server side.
		//
		// [...], in the event that the state of a resource changes in
		// a way that would cause a normal GET request at that time to return a
		// non-2.xx response (for example, when the resource is deleted), the
		// server SHOULD notify the client by sending a notification with an
		// appropriate response code (such as 4.04 Not Found) and subsequently
		// MUST remove the associated entry from the list of observers of the
		// resource.
		if resMsg.Code.IsError() {
			log.WithField("code", resMsg.Code.String()).Info("Stopped observer due to error response from server")
			// No need to send RST anymore but can't harm
			rst := coapmsg.NewRst(resMsg.MessageID)
			if err := sendMessage(ia.conn, &rst); err != nil {
				log.WithError(err).Error("Failed to send RST for notify (3)")
				return
			}
			return
		}
	}

	return
}

func validateMessageId(req, res *coapmsg.Message) error {
	if req.MessageID != res.MessageID {
		// This should never happen
		err := errors.New("coap: CRITICAL - MessageId of response does not match")
		log.WithError(err).
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
		log.WithError(err).
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
