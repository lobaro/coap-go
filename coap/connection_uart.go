package coap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Lobaro/slip"
)

// UartKeepAliveInterval defines how often the serial port is reopened.
// Set to 0 to disable reopening.
var UartKeepAliveInterval = 30 * time.Second

// UartUseSlipMux can be set to true to use SlipMux instead of SLIP
var UartUseSlipMux = false

var UartFlushOnRead = false

type serialPortCb func(port SerialPort)

var onSerialPortOpen serialPortCb

// SetOnSerialPortOpenHandler allows to set a callback that is called when ever a serial port is opened
// it allows e.g. to adjust RTS and DTR lines, flush buffers or just get a reference to the port
func SetOnSerialPortOpenHandler(cb serialPortCb) {
	onSerialPortOpen = cb
}

type SerialPort interface {
	io.Reader
	io.Writer
	io.Closer
	ResetInputBuffer() error
	ResetOutputBuffer() error
	// SetDTR sets the modem status bit DataTerminalReady
	SetDTR(dtr bool) error
	// SetRTS sets the modem status bit RequestToSend
	SetRTS(rts bool) error
}

// TODO: Use this struct instead of the bug.st one
type SerialMode struct {
	BaudRate int      // The serial port bitrate (aka Baudrate)
	DataBits int      // Size of the character (must be 5, 6, 7 or 8)
	Parity   Parity   // Parity (see Parity type for more info)
	StopBits StopBits // Stop bits (see StopBits type for more info)
}

type serialConnection struct {
	Interactions
	mode     UartParams
	portName string
	reader   PacketReader
	writer   PacketWriter
	open     bool

	// Use reader and writer to interact with the port
	port SerialPort

	cancelReceiveLoop context.CancelFunc

	readMu  sync.Mutex // Guards the reader
	writeMu sync.Mutex // Guards the writer
}

var ERR_CONNECTION_CLOSED = errors.New("Connection is closed")

func newSerialConnection(portName string, mode UartParams) *serialConnection {
	return &serialConnection{
		portName: portName,
		mode:     mode,
	}
}

func (c *serialConnection) setPort(port SerialPort) {
	c.port = port
	if UartUseSlipMux {
		c.reader = NewSlipMuxReader(port)
		c.writer = NewSlipMuxWriter(port)
	} else {
		c.reader = slip.NewReader(port)
		c.writer = slip.NewWriter(port)
	}

}

func (c *serialConnection) Name() string {
	return c.portName
}

func (c *serialConnection) Open() error {
	// TODO: not sure what happens when we reopen a closed connection
	oldName := c.portName
	port, newPortName, err := openComPort(c.portName, c.mode)
	c.portName = newPortName
	log.WithField("originalPort", oldName).
		WithField("port", c.portName).
		WithField("baud", c.mode.Baud).
		Info("Opening serial port ...")

	if err != nil {
		return wrapError(err, strings.TrimSpace("Failed to open serial port "+newPortName))
	}

	if onSerialPortOpen != nil {
		onSerialPortOpen(port)
	}

	c.setPort(port)
	c.open = true // Now we can actually send and receive data

	receiveLoopCtx, cancelReceiveLoop := context.WithCancel(context.Background())
	c.cancelReceiveLoop = cancelReceiveLoop
	go receiveLoop(receiveLoopCtx, c)
	go c.keepAlive()
	return nil
}

func (c *serialConnection) keepAlive() {
	for {
		if UartKeepAliveInterval == 0 {
			time.Sleep(10 * time.Second)
			continue
		}

		time.Sleep(UartKeepAliveInterval)
		if c.Closed() {
			log.Info("Serial port closed. Stop keep alive.")
			return
		}

		err := c.reopenSerialPort()
		if err != nil {
			log.WithError(err).Error("Failed to reopen serial port. Closing connection.")

			err := c.Close()
			if err != nil {
				log.WithError(err).Error("Failed to close connection.")
			}

			return
		}

	}
}

func (c *serialConnection) reopenSerialPort() error {
	//c.readMu.Lock()
	c.writeMu.Lock()
	//defer c.readMu.Unlock()
	defer c.writeMu.Unlock()

	log.WithField("port", c.portName).Info("Reopen serial port")
	// Close and reopen serial port
	err := c.port.Close()
	if err != nil {
		return wrapError(err, "Failed to close serial port")
	}
	c.port = nil
	// Need to wait a short period before reopening the port. Else it fails.
	time.Sleep(50 * time.Millisecond)
	log.WithField("port", c.portName).Debug("Port closed.")

	port, _, err := openComPort(c.portName, c.mode)
	if err != nil {
		return err
	}

	log.WithField("port", c.portName).Debug("Port opened.")

	c.setPort(port)

	return nil
}

func (c *serialConnection) ReadPacket() (p []byte, isPrefix bool, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if c.Closed() {
		err = ERR_CONNECTION_CLOSED
		return
	}

	p, isPrefix, err = c.reader.ReadPacket()

	if !isPrefix && UartFlushOnRead {
		log.Debug("Flush on ReadPacket")
		err = c.port.ResetInputBuffer()
		if err != nil {
			return
		}
		err = c.port.ResetOutputBuffer()
		if err != nil {
			return
		}
	}

	return
}

func (c *serialConnection) WritePacket(p []byte) (err error) {

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.Closed() {
		err = ERR_CONNECTION_CLOSED
		return
	}

	// We must NOT flush before writing, since it would cancel ongoing receiving at least on windows
	if err != nil {
		return
	}
	err = c.writer.WritePacket(p)

	return
}

func (c *serialConnection) Close() (err error) {
	c.open = false

	c.cancelReceiveLoop()
	if c.port != nil {
		err = c.port.Close()
	}
	return
}

func (c *serialConnection) Closed() bool {
	return !c.open
}

// Last successful "any" port. Will be tried first before iterating
var lastAny = ""

func checkComPort(portName string, mode UartParams) bool {
	port, err := serialOpen(portName, mode)
	if err != nil {
		return false
	} else {
		port.Close()
		return true
	}
}

// portlist generates a list of possible serial port names
func portlist() []string {
	ports := make([]string, 0)

	if isWindows() {
		for i := 0; i < 99; i++ {
			ports = append(ports, fmt.Sprintf("COM%d", i))
		}
	} else {
		for i := 0; i < 10; i++ {
			ports = append(ports, fmt.Sprintf("/dev/ttyUSB%d", i))
		}
		for i := 0; i < 99; i++ {
			ports = append(ports, fmt.Sprintf("/dev/tty%d", i))
		}
		for i := 0; i < 99; i++ {
			ports = append(ports, fmt.Sprintf("/dev/ttyS%d", i))
		}
	}
	return ports
}

// When portName is "any" the first available port is opened
// the new port name is returned as newPortName
func openComPort(portName string, mode UartParams) (port SerialPort, newPortName string, err error) {
	if portName == "any" {
		portNames, err := getPortsList()
		if err != nil {
			return nil, portName, err
		}

		for _, p := range portNames {
			if checkComPort(p, mode) {
				newPortName = p
				break
			} else {
				log.WithField("port", p).Info("Failed to use serial port")
			}
		}
	} else {
		newPortName = portName
	}

	if newPortName == "" {
		return nil, newPortName, errors.New("No usable serial port found")
	}

	start := time.Now()
	for {
		port, err = serialOpen(newPortName, mode)
		if err == nil {

			break
		}

		if time.Since(start) > time.Second {
			log.WithError(err).Debug("Failed to open serial port after 1 second")
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	err = port.ResetInputBuffer()
	if err != nil {
		return
	}
	err = port.ResetOutputBuffer()
	if err != nil {
		return
	}
	// err = port.Flush()
	return
}
