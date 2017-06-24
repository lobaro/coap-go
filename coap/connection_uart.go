package coap

import (
	"context"
	"errors"
	"sync"
	"time"

	"io"

	"github.com/Lobaro/slip"
	"go.bug.st/serial.v1"
)

var UartKeepAliveInterval = 30 * time.Second

type serialPort interface {
	io.Reader
	io.Writer
	io.Closer
	ResetInputBuffer() error
	ResetOutputBuffer() error
}

type serialConnection struct {
	Interactions
	mode     *serial.Mode
	portName string
	reader   PacketReader
	writer   PacketWriter
	open     bool

	// Use reader and writer to interact with the port
	port serialPort

	cancelReceiveLoop context.CancelFunc

	readMu  sync.Mutex // Guards the reader
	writeMu sync.Mutex // Guards the writer
}

var ERR_CONNECTION_CLOSED = errors.New("Connection is closed")

func newSerialConnection(portName string, mode *serial.Mode) *serialConnection {
	if mode == nil {
		panic("serial config must not be nil")
	}
	return &serialConnection{
		portName: portName,
		mode:     mode,
	}
}

func (c *serialConnection) setPort(port serial.Port) {
	c.port = port
	c.reader = slip.NewReader(port)
	c.writer = slip.NewWriter(port)
}

func (c *serialConnection) Open() error {
	// TODO: not sure what happens when we reopen a closed connection
	oldName := c.portName
	port, newPortName, err := openComPort(c.portName, c.mode)
	c.portName = newPortName
	log.WithField("originalPort", oldName).
		WithField("port", c.portName).
		WithField("baud", c.mode.BaudRate).
		Info("Opening serial port ...")

	if err != nil {
		return wrapError(err, "Failed to open serial port "+newPortName)
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

	if !isPrefix {
		log.Info("Flush on ReadPacket")
		err = c.port.ResetInputBuffer()
		//err = c.port.Flush()
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

// When portName is "any" the first available port is opened
// the new port name is returned as newPortName
func openComPort(portName string, mode *serial.Mode) (port serial.Port, newPortName string, err error) {
	// Search for serial port. Not needed for bugst/serial
	//
	// if serialCfg.Name == "any" {
	// 	if lastAny != "" {
	// 		serialCfg.Name = lastAny
	// 		port, err = serial.OpenPort(serialCfg)
	// 		if err == nil {
	// 			return
	// 		}
	// 	}
	// 	if isWindows() {
	// 		for i := 0; i < 99; i++ {
	// 			serialCfg.Name = fmt.Sprintf("COM%d", i)
	// 			port, err = serial.OpenPort(serialCfg)
	// 			if err == nil {
	// 				lastAny = serialCfg.Name
	// 				//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
	// 				return
	// 			}
	//
	// 		}
	// 	} else {
	// 		for i := 0; i < 99; i++ {
	// 			serialCfg.Name = fmt.Sprintf("/dev/tty%d", i)
	// 			port, err = serial.OpenPort(serialCfg)
	// 			if err == nil {
	// 				lastAny = serialCfg.Name
	// 				//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
	// 				return
	// 			}
	// 		}
	// 		for i := 0; i < 99; i++ {
	// 			serialCfg.Name = fmt.Sprintf("/dev/ttyS%d", i)
	// 			port, err = serial.OpenPort(serialCfg)
	// 			if err == nil {
	// 				lastAny = serialCfg.Name
	// 				//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
	// 				return
	// 			}
	// 		}
	// 		for i := 0; i < 10; i++ {
	// 			serialCfg.Name = fmt.Sprintf("/dev/ttyUSB%d", i)
	// 			port, err = serial.OpenPort(serialCfg)
	// 			if err == nil {
	// 				lastAny = serialCfg.Name
	// 				//logrus.WithField("comport", serialCfg.Name).Info("Resolved host 'any'")
	// 				return
	// 			}
	// 		}
	// 	}
	//
	// 	err = errors.New(fmt.Sprint("coap: Failed to find usable serial port: ", err.Error()))
	// 	return
	// }
	if portName == "any" {
		portNames, err := serial.GetPortsList()
		if err != nil {
			return nil, portName, err
		}

		if len(portNames) > 0 {
			newPortName = portNames[0]
		}
	} else {
		newPortName = portName
	}

	start := time.Now()
	for {
		port, err = serial.Open(newPortName, mode)
		if err == nil {

			break
		}

		if time.Since(start) > time.Second {
			return
		}
		log.WithError(err).Debug("Failed to open serial port")
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
