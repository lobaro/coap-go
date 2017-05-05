// We use this options outside of the package.
// It's more similar to the http Header API
package coapmsg

import (
	"encoding/binary"
)

// Currently only used in tests to find options
type OptionDef struct {
	Number       OptionId
	MinLength    int
	MaxLength    int
	DefaultValue []byte // Or interface{} or OptionValue?
	Repeatable   bool
	Format       ValueFormat
}

func (o *OptionDef) Critical() bool {
	return uint16(o.Number)&1 != 0
}

// "Unsafe to forward" proxies will not forward unsafe options
func (o *OptionDef) UnSafe() bool {
	return uint16(o.Number)&uint16(2) != 0
}

// NoCacheKey only has a meaning for options that are Safe-to-Forward
func (o *OptionDef) NoCacheKey() bool {
	return bool((o.Number & 0x1e) == 0x1c)
}

type OptionValue []byte

var NilOption OptionValue = OptionValue{}

func (v OptionValue) IsSet() bool {
	return len(v) > 0
}

func (v OptionValue) IsNotSet() bool {
	return !v.IsSet()
}

// For signed values just convert the result
func (v OptionValue) AsUInt8() uint8 {
	if len(v) == 0 {
		return 0
	}
	return v[0]
}

// For signed values just convert the result
func (v OptionValue) AsUInt16() uint16 {
	if len(v) == 0 {
		return 0
	}
	val := v
	for len(val) < 2 {
		val = append(val, 0)
	}
	return binary.LittleEndian.Uint16(val)
}

// For signed values just convert the result
func (v OptionValue) AsUInt32() uint32 {
	if len(v) == 0 {
		return 0
	}
	val := v
	for len(val) < 4 {
		val = append(val, 0)
	}
	return binary.LittleEndian.Uint32(val)
}

// For signed values just convert the result
func (v OptionValue) AsUInt64() uint64 {

	if len(v) == 0 {
		return 0
	}
	val := v
	for len(val) < 8 {
		val = append(val, 0)
	}
	return binary.LittleEndian.Uint64(val)
}

func (v OptionValue) AsString() string {
	return string(v)
}

func (v OptionValue) AsBytes() []byte {
	return v
}

// A CoapOptions represents a option mapping
// keys to sets of values.
type CoapOptions map[OptionId][]OptionValue

// Add adds the key, value pair to the header.
// It appends to any existing values associated with key.
func (h CoapOptions) Add(key OptionId, value interface{}) error {
	v, err := optionValueToBytes(value)
	if err != nil {
		return err
	}
	h[key] = append(h[key], v)
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
	h[key] = []OptionValue{v}
	return nil
}

// Get gets the first value associated with the given key.
// If there are no values associated with the key, Get returns
// NilOption. Get is a convenience method. For more
// complex queries, access the map directly.
func (h CoapOptions) Get(key OptionId) OptionValue {
	if h == nil {
		return NilOption
	}
	v := h[key]
	if len(v) == 0 {
		return NilOption
	}
	return v[0]
}

// Del deletes the values associated with key.
func (h CoapOptions) Del(key OptionId) {
	delete(h, key)
}
