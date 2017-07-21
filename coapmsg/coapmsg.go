package coapmsg

// https://github.com/dustin/go-coap
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// COAPType represents the message type.
type COAPType uint8

const (
	// Confirmable messages require acknowledgements.
	Confirmable COAPType = 0
	// NonConfirmable messages do not require acknowledgements.
	NonConfirmable COAPType = 1
	// Acknowledgement is a message indicating a response to confirmable message.
	Acknowledgement COAPType = 2
	// Reset indicates a permanent negative acknowledgement.
	Reset COAPType = 3
)

var typeNames = [256]string{
	Confirmable:     "Confirmable",
	NonConfirmable:  "NonConfirmable",
	Acknowledgement: "Acknowledgement",
	Reset:           "Reset",
}

func init() {
	for i := range typeNames {
		if typeNames[i] == "" {
			typeNames[i] = fmt.Sprintf("Unknown (0x%x)", i)
		}
	}
}

func (t COAPType) String() string {
	return typeNames[t]
}

// COAPCode is the type used for both request and response codes.
type COAPCode uint8

// Request Codes
const (
	GET    COAPCode = 1 // 0.01
	POST   COAPCode = 2 // 0.02
	PUT    COAPCode = 3 // 0.03
	DELETE COAPCode = 4 // 0.04
)

// Response Codes
const (
	Empty                 COAPCode = 0   // 0.00
	Created               COAPCode = 65  // 2.01
	Deleted               COAPCode = 66  // 2.02
	Valid                 COAPCode = 67  // 2.03
	Changed               COAPCode = 68  // 2.04
	Content               COAPCode = 69  // 2.05
	BadRequest            COAPCode = 128 // 4.00
	Unauthorized          COAPCode = 129 // 4.01
	BadOption             COAPCode = 130 // 4.02
	Forbidden             COAPCode = 131 // 4.03
	NotFound              COAPCode = 132 // 4.04
	MethodNotAllowed      COAPCode = 133 // 4.05
	NotAcceptable         COAPCode = 134 // 4.06
	PreconditionFailed    COAPCode = 140 // 4.12
	RequestEntityTooLarge COAPCode = 141 // 4.13
	UnsupportedMediaType  COAPCode = 143 // 4.15
	InternalServerError   COAPCode = 160 // 5.00
	NotImplemented        COAPCode = 161 // 5.01
	BadGateway            COAPCode = 162 // 5.02
	ServiceUnavailable    COAPCode = 163 // 5.03
	GatewayTimeout        COAPCode = 164 // 5.04
	ProxyingNotSupported  COAPCode = 165 // 5.05
)

var codeNames = [256]string{
	GET:                   "GET",
	POST:                  "POST",
	PUT:                   "PUT",
	DELETE:                "DELETE",
	Empty:                 "Empty",
	Created:               "Created",
	Deleted:               "Deleted",
	Valid:                 "Valid",
	Changed:               "Changed",
	Content:               "Content",
	BadRequest:            "BadRequest",
	Unauthorized:          "Unauthorized",
	BadOption:             "BadOption",
	Forbidden:             "Forbidden",
	NotFound:              "NotFound",
	MethodNotAllowed:      "MethodNotAllowed",
	NotAcceptable:         "NotAcceptable",
	PreconditionFailed:    "PreconditionFailed",
	RequestEntityTooLarge: "RequestEntityTooLarge",
	UnsupportedMediaType:  "UnsupportedMediaType",
	InternalServerError:   "InternalServerError",
	NotImplemented:        "NotImplemented",
	BadGateway:            "BadGateway",
	ServiceUnavailable:    "ServiceUnavailable",
	GatewayTimeout:        "GatewayTimeout",
	ProxyingNotSupported:  "ProxyingNotSupported",
}

func init() {
	for i := range codeNames {
		if codeNames[i] == "" {
			codeNames[i] = fmt.Sprintf("Unknown (0x%x)", i)
		}
	}
}

func (c COAPCode) String() string {
	return codeNames[c]
}

// First 3 bits of the code [0, 7]
func (c COAPCode) Class() uint8 {
	return uint8(c) >> 5
}

// Last 5 bits of the code [0, 31]
func (c COAPCode) Detail() uint8 {
	return uint8(c) & (0xFF >> 3)
}

func (c COAPCode) Number() uint8 {
	return uint8(c)
}

func (c COAPCode) IsSuccess() bool {
	return c.Class() == 2
}

func (c COAPCode) IsError() bool {
	return c.Class() != 2
}

func BuildCode(class, detail uint8) COAPCode {
	return COAPCode((class << 5) | detail)
}

// Message encoding errors.
var (
	ErrInvalidTokenLen   = errors.New("invalid token length")
	ErrOptionTooLong     = errors.New("option is too long")
	ErrOptionGapTooLarge = errors.New("option gap too large")
)

// Message is a CoAP message.
type Message struct {
	Type      COAPType
	Code      COAPCode
	MessageID uint16

	Token, Payload []byte

	options CoapOptions
}

func NewMessage() Message {
	return Message{
		options: CoapOptions{},
	}
}

func NewAck(messageId uint16) Message {
	return Message{
		Type:      Acknowledgement,
		Code:      Empty,
		MessageID: messageId,
	}
}

func NewRst(messageId uint16) Message {
	return Message{
		Type:      Reset,
		Code:      Empty,
		MessageID: messageId,
	}
}

func (m *Message) String() string {
	str := fmt.Sprintf(`coap.Message{Code:"%s", Type:"%s", MsgId:%d, Token:%v, Options:"%s", Payload:"%s"}`, m.Code, m.Type, m.MessageID, m.Token, m.Options(), m.Payload)
	return str
}

func (m *Message) Options() CoapOptions {
	if m.options == nil {
		m.options = CoapOptions{}
	}
	return m.options
}

func (m *Message) SetOptions(o CoapOptions) {
	m.options = o
}

// IsConfirmable returns true if this message is confirmable.
func (m *Message) IsConfirmable() bool {
	return m.Type == Confirmable
}

// IsConfirmable returns true if this message is confirmable.
func (m *Message) IsNonConfirmable() bool {
	return m.Type == NonConfirmable
}

// Path gets the Path set on this message if any.
func (m *Message) Path() []string {
	var path []string
	if pathOpts, ok := m.options[URIPath]; ok {
		for _, o := range pathOpts.values {
			path = append(path, o.AsString())
		}
	}
	return path
}

// PathString gets a path as a / separated string.
func (m *Message) PathString() string {
	return strings.Join(m.Path(), "/")
}

// SetPathString sets a path by a / separated string.
func (m *Message) SetPathString(s string) {
	if len(s) == 0 {
		m.SetPath(make([]string, 0))
		return
	}

	s = strings.TrimLeft(s, "/")
	m.SetPath(strings.Split(s, "/"))
}

// SetPath updates or adds a URIPath attribute on this message.
func (m *Message) SetPath(s []string) {
	m.Options().Del(URIPath)
	for _, part := range s {
		m.Options().Add(URIPath, part)
	}
}

const (
	extoptByteCode   = 13
	extoptByteAddend = 13
	extoptWordCode   = 14
	extoptWordAddend = 269
	extoptError      = 15
)

// Fulfill the encoding.BinaryMarshaler interface
func (m *Message) MarshalBinary() ([]byte, error) {
	return m.MustMarshalBinary(), nil
}

// MarshalBinary produces the binary form of this Message.
func (m *Message) MustMarshalBinary() []byte {
	tmpbuf := []byte{0, 0}
	binary.BigEndian.PutUint16(tmpbuf, m.MessageID)

	/*
	     0                   1                   2                   3
	    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	   |Ver| T |  TKL  |      Code     |          Message ID           |
	   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	   |   Token (if any, TKL bytes) ...
	   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	   |   Options (if any) ...
	   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	   |1 1 1 1 1 1 1 1|    Payload (if any) ...
	   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	*/

	buf := bytes.Buffer{}
	buf.Write([]byte{
		(1 << 6) | (uint8(m.Type) << 4) | uint8(0xf&len(m.Token)),
		byte(m.Code),
		tmpbuf[0], tmpbuf[1],
	})
	buf.Write(m.Token)

	/*
	     0   1   2   3   4   5   6   7
	   +---------------+---------------+
	   |               |               |
	   |  Option Delta | Option Length |   1 byte
	   |               |               |
	   +---------------+---------------+
	   \                               \
	   /         Option Delta          /   0-2 bytes
	   \          (extended)           \
	   +-------------------------------+
	   \                               \
	   /         Option Length         /   0-2 bytes
	   \          (extended)           \
	   +-------------------------------+
	   \                               \
	   /                               /
	   \                               \
	   /         Option Value          /   0 or more bytes
	   \                               \
	   /                               /
	   \                               \
	   +-------------------------------+
	   See parseExtOption(), extendOption()
	   and writeOptionHeader() below for implementation details
	*/

	extendOpt := func(opt int) (int, int) {
		ext := 0
		if opt >= extoptByteAddend {
			if opt >= extoptWordAddend {
				ext = opt - extoptWordAddend
				opt = extoptWordCode
			} else {
				ext = opt - extoptByteAddend
				opt = extoptByteCode
			}
		}
		return opt, ext
	}

	writeOptHeader := func(delta, length int) {
		d, dx := extendOpt(delta)
		l, lx := extendOpt(length)

		buf.WriteByte(byte(d<<4) | byte(l))

		tmp := []byte{0, 0}
		writeExt := func(opt, ext int) {
			switch opt {
			case extoptByteCode:
				buf.WriteByte(byte(ext))
			case extoptWordCode:
				binary.BigEndian.PutUint16(tmp, uint16(ext))
				buf.Write(tmp)
			}
		}

		writeExt(d, dx)
		writeExt(l, lx)
	}

	options := m.Options()

	ids := optionsIds{}
	for id := range options {
		ids = append(ids, id)
	}
	sort.Sort(ids)

	prev := 0

	for _, id := range ids {
		if _, ok := options[id]; !ok {
			continue
		}
		for _, val := range options[id].values {
			writeOptHeader(int(id)-prev, val.Len())
			buf.Write(val.AsBytes())
			prev = int(id)
		}
	}

	if len(m.Payload) > 0 {
		buf.Write([]byte{0xff})
	}

	buf.Write(m.Payload)

	return buf.Bytes()
}

func ParseMessage(data []byte) (Message, error) {
	rv := Message{}
	return rv, rv.UnmarshalBinary(data)
}

// UnmarshalBinary parses the given binary slice as a Message.
func (m *Message) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("short packet")
	}

	if data[0]>>6 != 1 {
		return errors.New("invalid version")
	}

	m.Type = COAPType((data[0] >> 4) & 0x3)
	tokenLen := int(data[0] & 0xf)
	if tokenLen > 8 {
		return ErrInvalidTokenLen
	}

	m.Code = COAPCode(data[1])
	m.MessageID = binary.BigEndian.Uint16(data[2:4])

	if tokenLen > 0 {
		m.Token = make([]byte, tokenLen)
	}
	if len(data) < 4+tokenLen {
		return errors.New("truncated")
	}
	copy(m.Token, data[4:4+tokenLen])
	b := data[4+tokenLen:]
	prev := 0

	parseExtOpt := func(opt int) (int, error) {
		switch opt {
		case extoptByteCode:
			if len(b) < 1 {
				return -1, errors.New("truncated")
			}
			opt = int(b[0]) + extoptByteAddend
			b = b[1:]
		case extoptWordCode:
			if len(b) < 2 {
				return -1, errors.New("truncated")
			}
			opt = int(binary.BigEndian.Uint16(b[:2])) + extoptWordAddend
			b = b[2:]
		}
		return opt, nil
	}

	for len(b) > 0 {
		if b[0] == 0xff {

			b = b[1:]

			// The presence of a marker followed by a zero-length
			// payload MUST be processed as a message format error.
			if len(b) == 0 {
				return errors.New("Message format error: Payload marker (0xFF) followed by zero-length payload")
			}
			break
		}

		delta := int(b[0] >> 4)
		length := int(b[0] & 0x0f)

		if delta == extoptError || length == extoptError {
			return errors.New("unexpected extended option marker")
		}

		b = b[1:]

		delta, err := parseExtOpt(delta)
		if err != nil {
			return err
		}
		length, err = parseExtOpt(length)
		if err != nil {
			return err
		}

		if len(b) < length {
			return errors.New("truncated")
		}

		oid := OptionId(prev + delta)
		val := b[:length]
		def, ok := optionDefs[oid]
		if ok && (len(val) < def.MinLength || len(val) > def.MaxLength) {
			// Skip options with illegal value length (RFC7252 section 5.4.3 and 5.4.1.)
			if oid.Critical() {
				// MUST cause the return of a 4.02 (Bad Option)
				// MUST cause the message / response to be rejected
				return errors.New("Critical option with invalid length found")
			}
			// Upon reception, unrecognized options of class "elective" MUST be silently ignored.
		} else {
			m.Options().Add(oid, val)
		}

		b = b[length:]
		prev = int(oid)
	}
	m.Payload = b
	return nil
}
