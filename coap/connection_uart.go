package coap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"io"

	"github.com/Lobaro/slip"
	"github.com/tarm/serial"
)

type serialConnection struct {
	Interactions
	config   *serial.Config
	deadline time.Time
	reader   PacketReader
	writer   PacketWriter
	closed   bool

	// Use reader and writer to interact with the port
	port *serial.Port

	cancelReceiveLoop context.CancelFunc

	readMu  sync.Mutex // Guards the reader
	writeMu sync.Mutex // Guards the writer
}

var ERR_CONNECTION_CLOSED = errors.New("Connection is closed")

func newSerialConnection(config *serial.Config) *serialConnection {
	if config == nil {
		panic("serial config must not be nil")
	}
	return &serialConnection{
		config: config,
	}
}

func (c *serialConnection) setPort(port *serial.Port) {
	c.port = port
	c.reader = slip.NewReader(port)
	c.writer = slip.NewWriter(port)
}

func (c *serialConnection) Open() error {
	// TODO: not sure what happens when we reopen a closed connection

	c.deadline = time.Now().Add(UART_CONNECTION_TIMEOUT)

	log.WithField("port", c.config.Name).WithField("baud", c.config.Baud).Info("Opening serial port ...")
	port, err := openComPort(c.config)

	if err != nil {
		return wrapError(err, "Failed to open serial port")
	}

	c.setPort(port)
	c.closed = false // Now we can actually send and receive data

	go c.closeAfterDeadline()

	receiveLoopCtx, cancelReceiveLoop := context.WithCancel(context.Background())
	c.cancelReceiveLoop = cancelReceiveLoop
	go receiveLoop(receiveLoopCtx, c)
	go c.keepAlive()
	return nil
}

func (c *serialConnection) keepAlive() {
	for {
		time.Sleep(30 * time.Second)
		if c.closed {
			return
		}

		err := c.reopenSerialPort()
		if err != nil {
			log.WithError(err).Error("Failed to reopen serial port")
			return
		}

	}
}

func (c *serialConnection) reopenSerialPort() error {
	c.readMu.Lock()
	c.writeMu.Lock()
	defer c.readMu.Unlock()
	defer c.writeMu.Unlock()

	log.WithField("port", c.config.Name).Info("Reopen serial port")
	// Close and reopen serial port
	c.port.Close()

	var err error
	port, err := openComPort(c.config)
	if err != nil {
		c.Close()
		return err
	}

	c.setPort(port)

	return nil
}

func (c *serialConnection) ReadPacket() (p []byte, isPrefix bool, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if c.closed {
		err = ERR_CONNECTION_CLOSED
		return
	}

	p, isPrefix, err = c.reader.ReadPacket()
	if err == nil && err != io.EOF {
		c.resetDeadline()
	}
	return
}

func (c *serialConnection) WritePacket(p []byte) (err error) {

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.closed {
		err = ERR_CONNECTION_CLOSED
		return
	}

	err = c.writer.WritePacket(p)
	if err == nil && err != io.EOF {
		c.resetDeadline()
	}
	return
}

func (c *serialConnection) Close() error {
	c.closed = true

	c.cancelReceiveLoop()
	if c.port != nil {
		return c.port.Close()
	}
	return nil
}

func (c *serialConnection) Closed() bool {
	return c.closed
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
					log.WithError(err).WithField("Port", c.config.Name).Error("Failed to close Serial Port after deadline")
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
