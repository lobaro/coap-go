package coap

import (
	"sync"
	"time"

	"github.com/Lobaro/slip"
	"github.com/tarm/serial"
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
		Size:         0,
		ReadTimeout:  time.Millisecond * 500,
		StopBits:     Stop1,
	}
}

func (c *UartConnector) newSerialConfig() *serial.Config {
	return &serial.Config{
		Name:        "",
		Baud:        c.Baud,
		Parity:      serial.Parity(c.Parity),
		Size:        c.Size,
		ReadTimeout: c.ReadTimeout,
		StopBits:    serial.StopBits(c.StopBits),
	}
}

func (c *UartConnector) Connect(host string) (*serialConnection, error) {
	c.connectMutex.Lock()
	defer c.connectMutex.Unlock()

	serialCfg := c.newSerialConfig()
	if host == "any" {
		serialCfg.Name = host
	} else if !isWindows() {
		serialCfg.Name = "/dev/" + host
	} else {
		serialCfg.Name = host
	}

	// can recycle connection?
	for i, con := range c.connections {
		if con.Closed() {
			c.connections = deleteConnection(c.connections, i)
			continue
		}

		if c, ok := con.(*serialConnection); (ok && c.config.Name == serialCfg.Name) || serialCfg.Name == "any" {
			log.WithField("Port", c.config.Name).Info("Reuseing Serial Port")
			return c, nil
		}
	}

	port, err := openComPort(serialCfg)
	if err != nil {
		return nil, wrapError(err, "Failed to open serial port")
	}
	conn := &serialConnection{
		config:   serialCfg,
		port:     port,
		reader:   slip.NewReader(port),
		writer:   slip.NewWriter(port),
		deadline: time.Now().Add(UART_CONNECTION_TIMEOUT),
	}
	c.connections = append(c.connections, conn)
	conn.Open()
	return conn, nil
}
