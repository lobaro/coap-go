package coap

import (
	"errors"
	"fmt"
	"github.com/Lobaro/slip"
	"github.com/Sirupsen/logrus"
	"github.com/tarm/serial"
	"time"
)

type serialConnection struct {
	config   *serial.Config
	deadline time.Time
	reader   *slip.SlipReader
	writer   *slip.SlipWriter
	closed   bool

	// Use reader and writer to interact with the port
	port *serial.Port
}

func (c *serialConnection) ReadPacket() (p []byte, isPrefix bool, err error) {
	p, isPrefix, err = c.reader.ReadPacket()
	c.resetDeadline()
	return
}

func (c *serialConnection) WritePacket(p []byte) (err error) {
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
	c.deadline = time.Now().Add(CONNECTION_TIMEOUT)
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
