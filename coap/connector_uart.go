package coap

import (
	"sync"
)

var DefaultUartParams = UartParams{
	Baud:       115200,
	Parity:     ParityNone,
	StopBits:   Stop1,
	DataBits:   8,
	InitialDTR: true,  // Good default for Lobaro Hardware
	InitialRTS: false, // Good default for Lobaro Hardware
}

type UartParams struct {
	// UART parameters. In future we might want to configure this per port.
	Baud       int      // BaudRate
	Parity     Parity   // Parity is the bit to use and defaults to ParityNone (no parity bit).
	StopBits   StopBits // Number of stop bits to use. Default is 1 (1 stop bit).
	DataBits   int
	InitialDTR bool
	InitialRTS bool
}

type UartConnector struct {
	UartParams
	connectMutex sync.Mutex
	connections  []Connection
}

func NewUartConnecter() *UartConnector {
	return &UartConnector{
		UartParams:   DefaultUartParams,
		connectMutex: sync.Mutex{},
		connections:  make([]Connection, 0),
	}
}

func (c *UartConnector) Connect(host string) (Connection, error) {
	c.connectMutex.Lock()
	defer c.connectMutex.Unlock()

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
			log.WithField("Port", c.portName).Debug("Using already open serial port")
			return c, nil
		}
	}

	// Else open a new connection
	conn := newSerialConnection(portName, c.UartParams)
	c.connections = append(c.connections, conn)
	err := conn.Open()
	if err != nil {
		return conn, err
	}

	return conn, nil
}
