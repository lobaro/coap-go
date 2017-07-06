package coapmsg

import (
	"encoding/binary"
	"fmt"
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

type optionsIds []OptionId

// Len implements sort.Interface
func (o optionsIds) Len() int {
	return len(o)
}

// Less implements sort.Interface
func (o optionsIds) Less(i, j int) bool {
	return o[i] < o[j]
}

// Swap implements sort.Interface
func (o optionsIds) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

// Obsolete, just used in tests
type option struct {
	ID    OptionId
	Value interface{}
}

func decodeInt(b []byte) uint32 {
	tmp := []byte{0, 0, 0, 0}
	copy(tmp[4-len(b):], b)
	return binary.BigEndian.Uint32(tmp)
}

func (o option) ToBytes() []byte {
	v, err := optionValueToBytes(o.Value)
	if err != nil {
		panic(fmt.Errorf("Failed to marshal option %d: %s", o.ID, err))
	}

	return v
}
