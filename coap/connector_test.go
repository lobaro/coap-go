package coap

import (
	"bytes"
	"io"
	"time"

	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/Lobaro/slip"
)

type TestConnector struct {
	ReceiveBuf *bytes.Buffer // Data that is received by the client (connection reader)
	SendBuf    *bytes.Buffer // Data that is send by the client (connection writer)
	conn       *serialConnection
}

func NewTestConnector() *TestConnector {
	return &TestConnector{
		ReceiveBuf: &bytes.Buffer{},
		SendBuf:    &bytes.Buffer{},
	}
}

func (c *TestConnector) FakeReceiveData(data []byte) {
	c.conn.readMu.Lock()
	defer c.conn.readMu.Unlock()

	w := slip.NewWriter(c.ReceiveBuf)
	w.WritePacket(data)
}

func (c *TestConnector) FakeReceiveMessage(msg coapmsg.Message) error {
	c.conn.readMu.Lock()
	defer c.conn.readMu.Unlock()
	w := slip.NewWriter(c.ReceiveBuf)

	p := msg.MarshalBinary()

	w.WritePacket(p)
	return nil
}

func (c *TestConnector) WaitForSendMessage() (coapmsg.Message, error) {
	w := slip.NewReader(c.SendBuf)

	p := make([]byte, 0)

	for {
		tmp, isPrefix, err := w.ReadPacket()
		if err != nil && err != io.EOF {
			return coapmsg.NewMessage(), err
		}

		p = append(p, tmp...)

		if len(p) > 0 && !isPrefix {
			break
		}
	}
	return coapmsg.ParseMessage(p)
}

func (c *TestConnector) GetSendMessage() (coapmsg.Message, error) {
	c.conn.writeMu.Lock()
	defer c.conn.writeMu.Unlock()
	w := slip.NewReader(c.SendBuf)

	p, _, err := w.ReadPacket()
	if err != nil {
		return coapmsg.NewMessage(), err
	}
	return coapmsg.ParseMessage(p)
}

func (c *TestConnector) GetSendData() ([]byte, error) {
	c.conn.writeMu.Lock()
	defer c.conn.writeMu.Unlock()
	w := slip.NewReader(c.SendBuf)

	p, _, err := w.ReadPacket()
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (c *TestConnector) Connect(host string) (*serialConnection, error) {

	if c.conn != nil {
		return c.conn, nil
	}

	conn := &serialConnection{
		config:   nil,
		port:     nil,
		reader:   slip.NewReader(c.ReceiveBuf),
		writer:   slip.NewWriter(c.SendBuf),
		deadline: time.Now().Add(UART_CONNECTION_TIMEOUT),
	}

	conn.Open()

	c.conn = conn

	return conn, nil
}
