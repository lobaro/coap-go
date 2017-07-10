// We use this options outside of the package.
// It's more similar to the http Header API
package coapmsg

import (
	"encoding/binary"
	"fmt"
	"strings"
)

type Option struct {
	Id     OptionId
	values []OptionValue
}

func (o Option) Len() int {
	return len(o.values)
}

func (o Option) IsSet() bool {
	return o.Id != 0
}

// In case of multiple option values it returns the first
func (o Option) AsString() string {
	str := []string{}
	for _, v := range o.values {
		str = append(str, v.AsString())
	}
	return strings.Join(str, "")
}

// In case of multiple option values it returns the first
func (o Option) AsUInt8() uint8 {
	if len(o.values) > 0 {
		return o.values[0].AsUInt8()
	}
	return 0
}

// In case of multiple option values it returns the first
func (o Option) AsUInt16() uint16 {
	if len(o.values) > 0 {
		return o.values[0].AsUInt16()
	}
	return 0
}

// In case of multiple option values it returns the first
func (o Option) AsUInt32() uint32 {
	if len(o.values) > 0 {
		return o.values[0].AsUInt32()
	}
	return 0
}

// In case of multiple option values it returns the first
func (o Option) AsUInt64() uint64 {
	if len(o.values) > 0 {
		return o.values[0].AsUInt64()
	}
	return 0
}

// In case of multiple option values it returns the first
func (o Option) AsBytes() []byte {
	if len(o.values) > 0 {
		return o.values[0].AsBytes()
	}
	return []byte{}
}

func (o Option) IsNotSet() bool {
	return !o.IsSet()
}

type OptionValue struct {
	b     []byte
	isNil bool
}

// Pretty print option
func (o Option) String() string {
	def, ok := optionDefs[o.Id]
	strOpts := make([]string, 0)
	if ok {

		for _, v := range o.values {
			strOpts = append(strOpts, def.Format.PrettyPrint(v))
		}
	} else {
		for _, v := range o.values {
			strOpts = append(strOpts, fmt.Sprintf("%#v", v.AsBytes()))
		}

	}
	return fmt.Sprintf("[%s]", strings.Join(strOpts, ", "))
}

var NilOptionValue OptionValue = OptionValue{isNil: true}

// For signed values just convert the result
func (v OptionValue) AsUInt8() uint8 {
	if len(v.b) == 0 {
		return 0
	}
	return v.b[0]
}

// For signed values just convert the result
func (v OptionValue) AsUInt16() uint16 {
	if len(v.b) == 0 {
		return 0
	}

	buf := make([]byte, 2)
	copy(buf, v.b)
	return binary.LittleEndian.Uint16(buf)
}

// For signed values just convert the result
func (v OptionValue) AsUInt32() uint32 {
	if len(v.b) == 0 {
		return 0
	}
	buf := make([]byte, 4)
	copy(buf, v.b)
	return binary.LittleEndian.Uint32(buf)
}

// For signed values just convert the result
func (v OptionValue) AsUInt64() uint64 {
	if len(v.b) == 0 {
		return 0
	}

	buf := make([]byte, 8)
	copy(buf, v.b)
	return binary.LittleEndian.Uint64(buf)
}

func (v OptionValue) AsString() string {
	buf := make([]byte, len(v.b))
	copy(buf, v.b)
	return string(buf)
}

func (v OptionValue) AsBytes() []byte {
	buf := make([]byte, len(v.b))
	copy(buf, v.b)
	return buf
}
func (v OptionValue) Len() int {
	return len(v.b)
}

// A CoapOptions represents a option mapping
// keys to sets of values.
type CoapOptions map[OptionId]Option

// Add adds the key, value pair to the header.
// It appends to any existing values associated with key.
func (h CoapOptions) Add(key OptionId, value interface{}) error {
	v, err := optionValueToBytes(value)
	if err != nil {
		return err
	}

	opt := h[key]
	opt.Id = key
	opt.values = append(opt.values, OptionValue{v, false})
	h[key] = opt
	return nil
}

// Set sets the header entries associated with key to
// the single element value. It replaces any existing
// values associated with key.
func (h CoapOptions) Set(key OptionId, value interface{}) error {
	v, err := optionValueToBytes(value)
	if err != nil {
		return err
	}

	opt := h[key]
	opt.Id = key
	opt.values = []OptionValue{{v, false}}
	h[key] = opt
	return nil
}

// Get gets the first value associated with the given key.
// If there are no values associated with the key, Get returns
// NilOptionValue. Get is a convenience method. For more
// complex queries, access the map directly.
func (h CoapOptions) Get(key OptionId) Option {
	v, ok := h[key]
	if !ok {
		return Option{}
	}
	return v
}

// Del deletes the values associated with key.
func (h CoapOptions) Del(key OptionId) {
	delete(h, key)
}

// Clear deletes all options.
func (h CoapOptions) Clear() {
	for k := range h {
		delete(h, k)
	}
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
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid type for option type: %T (%v)", optVal, optVal)
	}

	return encodeInt(v), nil
}
