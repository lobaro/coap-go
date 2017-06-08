package coap

import (
	"sync"
	"time"

	"go.bug.st/serial.v1"
)

type UartConnector struct {
	connectMutex sync.Mutex
	connections  []Connection

	// UART parameters. In future we might want to configure this per port.
	Baud        int           // BaudRate
	ReadTimeout time.Duration // Total timeout
	Size        byte          // Size is the number of data bits. If 0, DefaultSize is used.
	Parity      Parity        // Parity is the bit to use and defaults to ParityNone (no parity bit).
	StopBits    StopBits      // Number of stop bits to use. Default is 1 (1 stop bit).
}

func NewUartConnecter() *UartConnector {
	return &UartConnector{
		connectMutex: sync.Mutex{},
		connections:  make([]Connection, 0),
		Baud:         115200,
		Parity:       ParityNone,
		Size:         0,                      // TODO: Unused?
		ReadTimeout:  time.Millisecond * 500, // TODO: Unused?
		StopBits:     Stop1,
	}
}

func serialStopBits(bits StopBits) serial.StopBits {
	switch bits {
	case Stop1:
		return serial.OneStopBit
	case Stop1Half:
		return serial.OnePointFiveStopBits
	case Stop2:
		return serial.TwoStopBits
	}
	log.Warnf("Unknown stop bit option %v, using default: OneStopBit", bits)
	return serial.OneStopBit
}

func serialParity(parity Parity) serial.Parity {
	switch parity {
	case ParityNone:
		return serial.NoParity
	case ParityOdd:
		return serial.OddParity
	case ParityEven:
		return serial.EvenParity
	case ParityMark:
		return serial.MarkParity
	case ParitySpace:
		return serial.SpaceParity
	}
	log.Warnf("Unknown parity option %v, using default: NoParity", parity)
	return serial.NoParity
}

func (c *UartConnector) newSerialMode() *serial.Mode {
	return &serial.Mode{
		BaudRate: c.Baud,
		Parity:   serialParity(c.Parity),
		StopBits: serialStopBits(c.StopBits),
		DataBits: 8,
	}
}

func (c *UartConnector) Connect(host string) (Connection, error) {
	c.connectMutex.Lock()
	defer c.connectMutex.Unlock()

	serialMode := c.newSerialMode()
	portName := ""
	if host == "any" {
		portName = host
	} else if !isWindows() {
		portName = "/dev/" + host
	} else {
		portName = host
	}

	// can recycle connection?
	for i, con := range c.connections {
		if con.Closed() {
			c.connections = deleteConnection(c.connections, i)
			continue
		}

		if c, ok := con.(*serialConnection); (ok && c.portName == portName) || portName == "any" {
			// TODO: Should we force a reopen or flush here? It already happened that we received old garbage.
			log.WithField("Port", c.portName).Info("Reuseing Serial Port")
			return c, nil
		}
	}

	// Else open a new connection
	conn := newSerialConnection(portName, serialMode)
	c.connections = append(c.connections, conn)
	err := conn.Open()
	if err != nil {
		return conn, err
	}

	return conn, nil
}
