package coap

import (
	"context"
	"errors"
	"fmt"
	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/Lobaro/slip"
	"github.com/Sirupsen/logrus"
	"github.com/tarm/serial"
	"sync"
	"time"
)

type serialConnection struct {
	config       *serial.Config
	deadline     time.Time
	reader       *slip.SlipReader
	writer       *slip.SlipWriter
	closed       bool
	interactions Interactions

	// Use reader and writer to interact with the port
	port *serial.Port

	readMu  sync.Mutex
	writeMu sync.Mutex
}

func (c *serialConnection) ReadPacket() (p []byte, isPrefix bool, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	p, isPrefix, err = c.reader.ReadPacket()
	c.resetDeadline()
	return
}

func (c *serialConnection) WritePacket(p []byte) (err error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	err = c.writer.WritePacket(p)
	c.resetDeadline()
	return
}

func (c *serialConnection) Close() error {
	c.closed = true
	return c.port.Close()
}

func (c *serialConnection) Closed() bool {
	return c.closed
}

func (c *serialConnection) AddInteraction(ia *Interaction) {
	c.interactions = append(c.interactions, ia)
}

func (c *serialConnection) FindInteraction(token Token, msgId MessageId) (*Interaction, error) {
	for _, ia := range c.interactions {
		if ia.token.Equals(token) {
			return ia, nil
		}
		// For empty tokens the message Id must match
		// TODO: Check message type, for Con and Non we must not match by MessageId
		if len(token) == 0 && ia.MessageId == msgId {
			return ia, nil
		}
	}
	return nil, errors.New("Not Found")
}

func (c *serialConnection) closeAfterDeadline() {
	for {
		select {
		case now := <-time.After(c.deadline.Sub(time.Now())):
			if c.closed {
				return
			}

			if now.Equal(c.deadline) || now.After(c.deadline) {
				err := c.Close()
				if err != nil {
					logrus.WithError(err).WithField("Port", c.config.Name).Error("Failed to close Serial Port")
				} else {
					logrus.WithField("Port", c.config.Name).Info("Serial Port closed after deadline")
				}
				return
			}
		}
	}
}

func (c *serialConnection) resetDeadline() {
	c.deadline = time.Now().Add(UART_CONNECTION_TIMEOUT)
}

// Last successful "any" port. Will be tried first before iterating
var lastAny = ""

// Does change the config in case on Name == "any"
func openComPort(serialCfg *serial.Config) (port *serial.Port, err error) {

	if serialCfg.Name == "any" {
		if lastAny != "" {
			serialCfg.Name = lastAny
			port, err = serial.OpenPort(serialCfg)
			if err == nil {
				return
			}
		}
		if isWindows() {
			for i := 0; i < 99; i++ {
				serialCfg.Name = fmt.Sprintf("COM%d", i)
				port, err = serial.OpenPort(serialCfg)
				if err == nil {
					lastAny = serialCfg.Name
					//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
					return
				}

			}
		} else {
			for i := 0; i < 99; i++ {
				serialCfg.Name = fmt.Sprintf("/dev/ttyS%d", i)
				port, err = serial.OpenPort(serialCfg)
				if err == nil {
					lastAny = serialCfg.Name
					//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
					return
				}

			}
		}

		err = errors.New(fmt.Sprint("coap: Failed to find usable serial port: ", err.Error()))
		return
	}
	port, err = serial.OpenPort(serialCfg)
	return
}

func (c *serialConnection) StartReceiveLoop(ctx context.Context) {
	for {
		msg, err := readMessage(ctx, c)

		if err != nil {
			logrus.WithError(err).Error("Failed to receive message - closing connection")
			c.Close()
			return
		}

		ia, err := c.FindInteraction(Token(msg.Token), MessageId(msg.MessageID))
		if err != nil {
			logrus.WithError(err).
				WithField("token", msg.Token).
				WithField("messageId", msg.MessageID).
				Warn("Failed to find interaction, send RST and drop packet")

			// Even non-confirmable messages can be answered with a RST
			rst := coapmsg.NewRst(msg.MessageID)
			if err := sendMessage(c, &rst); err != nil {
				logrus.WithError(err).Warn("Failed to send RST")
			}
		} else {
			// ACK must be handled by interaction!
			// Only ACK con messages
			// if msg.Type == coapmsg.Confirmable {
			// 	ack := coapmsg.NewAck(msg.MessageID)
			// 	if err := sendMessage(c, &ack); err != nil {
			// 		logrus.WithError(err).Warn("Failed to send ACK")
			// 	}
			// }

			ia.HandleMessage(msg)
		}

	}
}
