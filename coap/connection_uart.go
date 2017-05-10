package coap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"io"

	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/tarm/serial"
)

type serialConnection struct {
	config       *serial.Config
	deadline     time.Time
	reader       PacketReader
	writer       PacketWriter
	closed       bool
	interactions Interactions

	// Use reader and writer to interact with the port
	port *serial.Port

	readMu  sync.Mutex // Guards the reader
	writeMu sync.Mutex // Guards the writer
}

func (c *serialConnection) Open() error {
	// TODO: not sure what happens when we reopen a closed connection
	c.closed = false
	go c.closeAfterDeadline()
	go c.startReceiveLoop(context.Background())
	return nil
}

func (c *serialConnection) ReadPacket() (p []byte, isPrefix bool, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	p, isPrefix, err = c.reader.ReadPacket()
	if err == nil && err != io.EOF {
		c.resetDeadline()
	}
	return
}

func (c *serialConnection) WritePacket(p []byte) (err error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	err = c.writer.WritePacket(p)
	if err == nil && err != io.EOF {
		c.resetDeadline()
	}
	return
}

func (c *serialConnection) Close() error {
	c.closed = true
	if c.port != nil {
		return c.port.Close()
	}
	return nil
}

func (c *serialConnection) Closed() bool {
	return c.closed
}

func (c *serialConnection) AddInteraction(ia *Interaction) {
	c.interactions = append(c.interactions, ia)
}

func (c *serialConnection) RemoveInteraction(interaction *Interaction) {
	for i, ia := range c.interactions {
		if ia == interaction {
			copy(c.interactions[i:], c.interactions[i+1:])
			c.interactions[len(c.interactions)-1] = nil // or the zero value of T
			c.interactions = c.interactions[:len(c.interactions)-1]
			return
		}
	}
}

func (c *serialConnection) FindInteraction(token Token, msgId MessageId) *Interaction {
	for _, ia := range c.interactions {
		if ia.Token().Equals(token) {
			return ia
		}
		// For empty tokens the message Id must match
		// An ACK is sent by the server to confirm a CON but carries no token
		// TODO: Check also message type?
		if len(token) == 0 && ia.lastMessageId == msgId {
			return ia
		}
	}
	return nil
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
					log.WithError(err).WithField("Port", c.config.Name).Error("Failed to close Serial Port")
				} else {
					log.WithField("Port", c.config.Name).Info("Serial Port closed after deadline")
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
				serialCfg.Name = fmt.Sprintf("/dev/tty%d", i)
				port, err = serial.OpenPort(serialCfg)
				if err == nil {
					lastAny = serialCfg.Name
					//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
					return
				}
			}
			for i := 0; i < 99; i++ {
				serialCfg.Name = fmt.Sprintf("/dev/ttyS%d", i)
				port, err = serial.OpenPort(serialCfg)
				if err == nil {
					lastAny = serialCfg.Name
					//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
					return
				}
			}
			for i := 0; i < 10; i++ {
				serialCfg.Name = fmt.Sprintf("/dev/ttyUSB%d", i)
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

func (c *serialConnection) startReceiveLoop(ctx context.Context) {
	for {
		//log.Info("Receive loop")
		msg, err := readMessage(ctx, c)

		if err != nil {
			log.WithError(err).Error("Failed to receive message - closing connection")
			c.Close()
			return
		}

		ia := c.FindInteraction(Token(msg.Token), MessageId(msg.MessageID))
		if ia == nil {
			log.WithError(err).
				WithField("token", msg.Token).
				WithField("messageId", msg.MessageID).
				Warn("Failed to find interaction, send RST and drop packet")

			// Even non-confirmable messages can be answered with a RST
			rst := coapmsg.NewRst(msg.MessageID)
			if err := sendMessage(c, &rst); err != nil {
				log.WithError(err).Warn("Failed to send RST")
			}
		} else {
			ia.HandleMessage(msg)
		}

	}
}
