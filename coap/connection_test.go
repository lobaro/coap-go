package coap

import (
	"context"
	"sync"
)

type TestConnection struct {
	Interactions

	reader PacketReader
	writer PacketWriter
	closed bool

	cancelReceiveLoop context.CancelFunc

	readMu  sync.Mutex // Guards the reader
	writeMu sync.Mutex // Guards the writer
}

func NewTestConnection(reader PacketReader, writer PacketWriter) *TestConnection {
	return &TestConnection{
		reader: reader,
		writer: writer,
	}
}

func (c *TestConnection) Name() string {
	return "TestConnection"
}

func (c *TestConnection) Open() error {
	c.closed = false

	receiveLoopCtx, cancelReceiveLoop := context.WithCancel(context.Background())
	c.cancelReceiveLoop = cancelReceiveLoop
	go receiveLoop(receiveLoopCtx, c)

	return nil
}

func (c *TestConnection) Close() error {
	c.closed = true

	c.cancelReceiveLoop()

	return nil
}

func (c *TestConnection) Closed() bool {
	return c.closed
}

func (c *TestConnection) ReadPacket() (p []byte, isPrefix bool, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	return c.reader.ReadPacket()
}

func (c *TestConnection) WritePacket(p []byte) (err error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.writer.WritePacket(p)
}
