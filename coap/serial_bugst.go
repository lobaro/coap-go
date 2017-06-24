package coap

import "go.bug.st/serial.v1"

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
