package coapmsg

import (
	"encoding/binary"
	"fmt"
)

// OptionID identifies an option in a message.
type OptionID uint16

/*
   +-----+----+---+---+---+----------------+--------+--------+---------+
   | No. | C  | U | N | R | Name           | Format | Length | Default |
   +-----+----+---+---+---+----------------+--------+--------+---------+
   |   1 | x  |   |   | x | If-Match       | opaque | 0-8    | (none)  |
   |   3 | x  | x | - |   | Uri-Host       | string | 1-255  | (see    |
   |     |    |   |   |   |                |        |        | below)  |
   |   4 |    |   |   | x | ETag           | opaque | 1-8    | (none)  |
   |   5 | x  |   |   |   | If-None-Match  | empty  | 0      | (none)  |
   |   7 | x  | x | - |   | Uri-Port       | uint   | 0-2    | (see    |
   |     |    |   |   |   |                |        |        | below)  |
   |   8 |    |   |   | x | Location-Path  | string | 0-255  | (none)  |
   |  11 | x  | x | - | x | Uri-Path       | string | 0-255  | (none)  |
   |  12 |    |   |   |   | Content-Format | uint   | 0-2    | (none)  |
   |  14 |    | x | - |   | Max-Age        | uint   | 0-4    | 60      |
   |  15 | x  | x | - | x | Uri-Query      | string | 0-255  | (none)  |
   |  17 | x  |   |   |   | Accept         | uint   | 0-2    | (none)  |
   |  20 |    |   |   | x | Location-Query | string | 0-255  | (none)  |
   |  35 | x  | x | - |   | Proxy-Uri      | string | 1-1034 | (none)  |
   |  39 | x  | x | - |   | Proxy-Scheme   | string | 1-255  | (none)  |
   |  60 |    |   | x |   | Size1          | uint   | 0-4    | (none)  |
   +-----+----+---+---+---+----------------+--------+--------+---------+
*/

// Option IDs.
const (
	IfMatch       OptionID = 1
	URIHost       OptionID = 3
	ETag          OptionID = 4
	IfNoneMatch   OptionID = 5
	Observe       OptionID = 6
	URIPort       OptionID = 7
	LocationPath  OptionID = 8
	URIPath       OptionID = 11
	ContentFormat OptionID = 12
	MaxAge        OptionID = 14
	URIQuery      OptionID = 15
	Accept        OptionID = 17
	LocationQuery OptionID = 20
	ProxyURI      OptionID = 35
	ProxyScheme   OptionID = 39
	Size1         OptionID = 60
)

// MediaType specifies the content type of a message.
type MediaType byte

// Content types.
const (
	TextPlain     MediaType = 0  // text/plain;charset=utf-8
	AppLinkFormat MediaType = 40 // application/link-format
	AppXML        MediaType = 41 // application/xml
	AppOctets     MediaType = 42 // application/octet-stream
	AppExi        MediaType = 47 // application/exi
	AppJSON       MediaType = 50 // application/json
)

// Option value format (RFC7252 section 3.2)
type valueFormat uint8

const (
	valueUnknown valueFormat = iota
	valueEmpty
	valueOpaque
	valueUint
	valueString
)

type optionDef struct {
	valueFormat valueFormat
	minLen      int
	maxLen      int
}

var optionDefs = [256]optionDef{
	IfMatch:       {valueFormat: valueOpaque, minLen: 0, maxLen: 8},
	URIHost:       {valueFormat: valueString, minLen: 1, maxLen: 255},
	ETag:          {valueFormat: valueOpaque, minLen: 1, maxLen: 8},
	IfNoneMatch:   {valueFormat: valueEmpty, minLen: 0, maxLen: 0},
	Observe:       {valueFormat: valueUint, minLen: 0, maxLen: 3},
	URIPort:       {valueFormat: valueUint, minLen: 0, maxLen: 2},
	LocationPath:  {valueFormat: valueString, minLen: 0, maxLen: 255},
	URIPath:       {valueFormat: valueString, minLen: 0, maxLen: 255},
	ContentFormat: {valueFormat: valueUint, minLen: 0, maxLen: 2},
	MaxAge:        {valueFormat: valueUint, minLen: 0, maxLen: 4},
	URIQuery:      {valueFormat: valueString, minLen: 0, maxLen: 255},
	Accept:        {valueFormat: valueUint, minLen: 0, maxLen: 2},
	LocationQuery: {valueFormat: valueString, minLen: 0, maxLen: 255},
	ProxyURI:      {valueFormat: valueString, minLen: 1, maxLen: 1034},
	ProxyScheme:   {valueFormat: valueString, minLen: 1, maxLen: 255},
	Size1:         {valueFormat: valueUint, minLen: 0, maxLen: 4},
}

type options []option

func (o options) Len() int {
	return len(o)
}

func (o options) Less(i, j int) bool {
	if o[i].ID == o[j].ID {
		return i < j
	}
	return o[i].ID < o[j].ID
}

func (o options) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func (o options) Remove(oid OptionID) options {
	rv := options{}
	for _, opt := range o {
		if opt.ID != oid {
			rv = append(rv, opt)
		}
	}
	return rv
}

type option struct {
	ID    OptionID
	Value interface{}
}

func encodeInt(v uint32) []byte {
	switch {
	case v == 0:
		return nil
	case v < 256:
		return []byte{byte(v)}
	case v < 65536:
		rv := []byte{0, 0}
		binary.BigEndian.PutUint16(rv, uint16(v))
		return rv
	case v < 16777216:
		rv := []byte{0, 0, 0, 0}
		binary.BigEndian.PutUint32(rv, uint32(v))
		return rv[1:]
	default:
		rv := []byte{0, 0, 0, 0}
		binary.BigEndian.PutUint32(rv, uint32(v))
		return rv
	}
}

func decodeInt(b []byte) uint32 {
	tmp := []byte{0, 0, 0, 0}
	copy(tmp[4-len(b):], b)
	return binary.BigEndian.Uint32(tmp)
}

func (o option) ToBytes() []byte {
	v, err := optionValueToBytes(o.Value)
	if err != nil {
		panic(fmt.Errorf("Failed to marshal option %x: %s", o.ID, err))
	}

	return v
}

func optionValueToBytes(optVal interface{}) ([]byte, error) {
	var v uint32

	switch i := optVal.(type) {
	case string:
		return []byte(i), nil
	case []byte:
		return i, nil
	case MediaType:
		v = uint32(i)
	case int:
		v = uint32(i)
	case int16:
		v = uint32(i)
	case int32:
		v = uint32(i)
	case uint:
		v = uint32(i)
	case uint16:
		v = uint32(i)
	case uint32:
		v = i
	default:
		return nil, fmt.Errorf("invalid type for option type: %T (%v)", optVal, optVal)
	}

	return encodeInt(v), nil
}

func parseOptionValue(optionID OptionID, valueBuf []byte) interface{} {
	// Custom option?
	if int(optionID) > len(optionDefs) {
		return valueBuf
	}

	def := optionDefs[optionID]

	if def.valueFormat == valueUnknown {
		// Skip unrecognized options (RFC7252 section 5.4.1)
		return nil
	}
	if len(valueBuf) < def.minLen || len(valueBuf) > def.maxLen {
		// Skip options with illegal value length (RFC7252 section 5.4.3)
		return nil
	}
	switch def.valueFormat {
	case valueUint:
		intValue := decodeInt(valueBuf)
		if optionID == ContentFormat || optionID == Accept {
			return MediaType(intValue)
		} else {
			return intValue
		}
	case valueString:
		return string(valueBuf)
	case valueOpaque, valueEmpty:
		return valueBuf
	}
	// Skip unrecognized options (should never be reached)
	return nil
}
