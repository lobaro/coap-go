package coapmsg

import "fmt"

// Option value format (RFC7252 section 3.2)
// Defines the option format inside the packet
type ValueFormat uint8

const (
	ValueUnknown ValueFormat = iota
	ValueEmpty               // A zero-length sequence of bytes.
	ValueOpaque              // An opaque sequence of bytes.
	// A non-negative integer that is represented in network byte
	// order using the number of bytes given by the Option Length
	// field.
	ValueUint
	// A Unicode string that is encoded using UTF-8 [RFC3629] in
	// Net-Unicode form [RFC5198].
	ValueString
)

func (f ValueFormat) PrettyPrint(val OptionValue) string {
	switch f {
	case ValueUnknown:
		return fmt.Sprintf("?%#v", val.AsBytes())
	case ValueEmpty:
		return "-Empty-"
	case ValueOpaque:
		return fmt.Sprintf("0x%X", val.AsBytes())
	case ValueUint:
		return fmt.Sprintf("%d", val.AsUInt64())
	case ValueString:
		return fmt.Sprintf("'%s'", val.AsString())
	}

	return fmt.Sprintf("%#v", val.AsBytes())
}

// Currently only used in tests to find options
type OptionDef struct {
	Number       OptionId
	MinLength    int
	MaxLength    int
	DefaultValue []byte // Or interface{} or OptionValue?
	Repeatable   bool
	Format       ValueFormat
}

// Information about options used for handling the values
var optionDefs = map[OptionId]OptionDef{
	IfMatch:     {Format: ValueOpaque, MinLength: 0, MaxLength: 8},
	URIHost:     {Format: ValueString, MinLength: 1, MaxLength: 255},
	ETag:        {Format: ValueOpaque, MinLength: 1, MaxLength: 8},
	IfNoneMatch: {Format: ValueEmpty, MinLength: 0, MaxLength: 0},
	// Observe a resource for up to 256 Seconds
	Observe:       {Format: ValueUint, MinLength: 0, MaxLength: 3}, // Client: 0 = register, 1 = unregister; Server: Seq. number
	URIPort:       {Format: ValueUint, MinLength: 0, MaxLength: 2},
	LocationPath:  {Format: ValueString, MinLength: 0, MaxLength: 255},
	URIPath:       {Format: ValueString, MinLength: 0, MaxLength: 255},
	ContentFormat: {Format: ValueUint, MinLength: 0, MaxLength: 2},
	MaxAge:        {Format: ValueUint, MinLength: 0, MaxLength: 4},
	URIQuery:      {Format: ValueString, MinLength: 0, MaxLength: 255},
	Accept:        {Format: ValueUint, MinLength: 0, MaxLength: 2},
	LocationQuery: {Format: ValueString, MinLength: 0, MaxLength: 255},
	ProxyURI:      {Format: ValueString, MinLength: 1, MaxLength: 1034},
	ProxyScheme:   {Format: ValueString, MinLength: 1, MaxLength: 255},
	Size1:         {Format: ValueUint, MinLength: 0, MaxLength: 4},
}
