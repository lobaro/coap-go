package coap

import "github.com/Lobaro/go-serial"

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

func (c *UartParams) toBugstSerialMode() *serial.Mode {
	return &serial.Mode{
		BaudRate:   c.Baud,
		Parity:     serialParity(c.Parity),
		StopBits:   serialStopBits(c.StopBits),
		DataBits:   c.DataBits,
		InitialDTR: c.InitialDTR, //true,
		InitialRTS: c.InitialRTS, // false,
	}
}

func getPortsList() ([]string, error) {
	return serial.GetPortsList()
}

func serialOpen(portName string, params UartParams) (SerialPort, error) {
	return serial.Open(portName, params.toBugstSerialMode())
}
