package coap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/Lobaro/slip"
	"github.com/Sirupsen/logrus"
	"github.com/tarm/serial"
	"io"
	"io/ioutil"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"
)

type StopBits byte
type Parity byte

const (
	Stop1     StopBits = 1
	Stop1Half StopBits = 15
	Stop2     StopBits = 2
)

const (
	ParityNone  Parity = 'N'
	ParityOdd   Parity = 'O'
	ParityEven  Parity = 'E'
	ParityMark  Parity = 'M' // parity bit is always 1
	ParitySpace Parity = 'S' // parity bit is always 0
)

const UartScheme = "coap+uart"

// Transport uses a Serial port to communicate via UART (e.g. RS232)
// All Serial parameters can be set on the transport
// The host of the request URL specifies the serial connection, e.g. COM3
// The URI scheme must be coap+uart and valid URIs would be
// coap+uart://COM3/sensors/temperature
// coap+uart://ttyS2/sensors/temperature
// Since we can not have a slash (/) in the host name, on linux systems
// the /dev/ part of the device file handle is added implicitly
// https://tools.ietf.org/html/rfc3986#page-21 allows system specific Host lookups
//
// The URI host can be set to "any" to take the first open port found
type TransportUart struct {
	mu        *sync.Mutex
	lastMsgId uint16     // Sequence counter
	rand      *rand.Rand // Random source for token generation
	// TODO: add parameter for serial connection like Baud rate.

	Name        string
	Baud        int
	ReadTimeout time.Duration // Total timeout

	// Size is the number of data bits. If 0, DefaultSize is used.
	Size byte

	// Parity is the bit to use and defaults to ParityNone (no parity bit).
	Parity Parity

	// Number of stop bits to use. Default is 1 (1 stop bit).
	StopBits StopBits
}

func NewTransportUart() *TransportUart {
	return &TransportUart{
		mu:          &sync.Mutex{},
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
		Baud:        115200,
		Parity:      ParityNone,
		Size:        0,
		ReadTimeout: time.Millisecond * 500,
		StopBits:    Stop1,
	}

}

const ACK_RANDOM_FACTOR = 1.5
const ACK_TIMEOUT = 2 * time.Second

func logMsg(resMsg *coapmsg.Message) {
	logrus.WithField("Code", resMsg.Code.String()).
		WithField("Type", resMsg.Type.String()).
		WithField("Token", resMsg.Token).
		WithField("MessageID", resMsg.MessageID).
		WithField("Payload", resMsg.Payload).
		WithField("OptionCount", resMsg.OptionsRaw().Len()).
		Info("Got response message")
}

func (t *TransportUart) ackTimeout() time.Duration {
	return time.Duration(float64(ACK_TIMEOUT) * ACK_RANDOM_FACTOR)
}

func (t *TransportUart) RoundTrip(req *Request) (res *Response, err error) {

	if req == nil {
		return nil, errors.New("coap: Got nil request")
	}

	if !req.Confirmable {
		// TODO: Implement non-confirmable requests
		// This will need some concept of "interactions" matched via Message Ids and Tokens
		return nil, errors.New("coap: Non-Confirmable request not stupported yet!")
	}

	reqMsg, err := t.buildMessage(req)
	if err != nil {
		return
	}

	//###########################################
	// Open the connection and write the request
	//###########################################

	serialCfg := t.newSerialConfig()

	if req.URL == nil {
		return nil, errors.New(fmt.Sprint("coap: Missing request URL"))
	}
	if req.URL.Scheme != UartScheme {
		return nil, errors.New(fmt.Sprint("coap: Invalid URL scheme, expected "+UartScheme+" but got: ", req.URL.Scheme))
	}

	if req.URL.Host == "any" {
		serialCfg.Name = req.URL.Host
	} else if !isWindows() {
		serialCfg.Name = "/dev/" + req.URL.Host
	} else {
		serialCfg.Name = req.URL.Host
	}

	conn, err := openComPort(serialCfg)
	if err != nil {
		return
	}
	defer conn.Close()

	bin, err := reqMsg.MarshalBinary()
	if err != nil {
		return
	}

	slipWriter := slip.NewWriter(conn)
	slipWriter.WritePacket(bin)

	//###########################################
	// Read the response
	//###########################################

	// TODO: Implement retries until first ACK is received or some timeout
	ctx, _ := context.WithTimeout(req.Context(), t.ackTimeout())
	resMsg, err := readResponse(ctx, conn)
	if err != nil {
		return
	}

	if req.Confirmable && resMsg.MessageID != reqMsg.MessageID {
		return nil, errors.New("coap: MessageId of response does not match")
	}

	// TODO: Handle Types: Con and Reset correctly - now we just don't care
	// Handle postponed response
	if resMsg.Type == coapmsg.Acknowledgement && resMsg.Code == coapmsg.Empty {
		//  Client              Server
		//    |                  |
		//    |   CON [0x7a10]   |
		//    | GET /temperature |
		//    |   (Token 0x73)   |
		//    +----------------->|
		//    |                  |
		//    |   ACK [0x7a10]   | <- We are here!
		//    |<-----------------+
		//    |                  |
		//    ... Time Passes  ...
		//    |                  |
		//    |   CON [0x23bb]   |
		//    |   2.05 Content   |
		//    |   (Token 0x73)   |
		//    |     "22.5 C"     |
		//    |<-----------------+
		//    |                  |
		//    |   ACK [0x23bb]   |
		//    +----------------->|
		//    |                  |
		//
		// Figure 5: A GET Request with a Separate Response

		//ctx, _ := context.WithTimeout(req.Context(), 10 * time.Second)
		resMsg, err = readResponse(req.Context(), conn)
		if err != nil {
			return
		}
		logMsg(resMsg)
	}

	if !bytesAreEqual(reqMsg.Token, resMsg.Token) {
		return nil, errors.New(fmt.Sprintf("coap: Token of response does not match %x != %x", reqMsg.Token, resMsg.Token))
	}

	res = &Response{
		StatusCode: int(resMsg.Code),
		Status:     fmt.Sprintf("%d.%02d %s", resMsg.Code.Class(), resMsg.Code.Detail(), resMsg.Code.String()),
		Body:       ioutil.NopCloser(bytes.NewReader(resMsg.Payload)),
		Request:    req,
	}
	return res, nil
}
func readResponse(ctx context.Context, conn *serial.Port) (resMsg *coapmsg.Message, err error) {
	packetCh := make(chan []byte)
	errorCh := make(chan error)
	var packet []byte
	go func() {
		packet, err := readPacket(conn)
		if err != nil {
			errorCh <- err
		} else {
			packetCh <- packet
		}
		logrus.Info("ReadPacket Done!")
	}()

	select {
	case err = <-errorCh:
		return
	case packet = <-packetCh:
		break
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	msg, err := coapmsg.ParseMessage(packet)
	if err != nil {
		logrus.WithField("dataStr", string(packet)).Error("Failed to parse CoAP message")
		return nil, err
	}
	return &msg, nil
}

func readPacket(conn *serial.Port) ([]byte, error) {
	slipReader := slip.NewReader(conn)

	buf := &bytes.Buffer{}

	var isPrefix bool

	for {
		var p []byte
		p, isPrefix, err := slipReader.ReadPacket()
		buf.Write(p)

		if err != nil {
			return nil, err
		}

		if !isPrefix {
			break
		}
	}

	if isPrefix {
		return nil, errors.New("coap: Did read incomplete response")
	}

	return buf.Bytes(), nil
}

func bytesAreEqual(a, b []byte) bool {
	if len(a) != len(b) {

		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (t *TransportUart) newSerialConfig() *serial.Config {
	return &serial.Config{
		Name:        "",
		Baud:        t.Baud,
		Parity:      serial.Parity(t.Parity),
		Size:        t.Size,
		ReadTimeout: t.ReadTimeout,
		StopBits:    serial.StopBits(t.StopBits),
	}
}

// BuildMessage creates a coap message based on the request
// Takes care of closing the request body
func (t *TransportUart) buildMessage(req *Request) (*coapmsg.Message, error) {
	defer req.Body.Close()
	if !ValidMethod(req.Method) {
		return nil, errors.New(fmt.Sprint("coap: Invalid method: ", req.Method))
	}

	msgType := coapmsg.NonConfirmable
	if req.Confirmable {
		msgType = coapmsg.Confirmable
	}

	msg := &coapmsg.Message{
		Code:      methodToCode(req.Method),
		Type:      msgType,
		MessageID: t.nextMessageId(),
		Token:     t.nextToken(),
	}
	msg.SetOptions(req.Options)
	msg.SetPathString(req.URL.EscapedPath())

	for _, q := range strings.Split(req.URL.RawQuery, "&") {
		msg.SetOption(coapmsg.URIQuery, q)
	}

	buf := &bytes.Buffer{}
	n, err := buf.ReadFrom(req.Body)
	if n > 0 && err != nil && err != io.EOF {
		return nil, err
	}
	msg.Payload = buf.Bytes()

	// Gracefully close the body instead of waiting for the defer
	if err := req.Body.Close(); err != nil {
		return nil, err
	}

	return msg, nil
}

func (t *TransportUart) nextMessageId() uint16 {
	t.mu.Lock()
	t.lastMsgId++
	msgId := t.lastMsgId
	t.mu.Unlock()
	return msgId
}

func (t *TransportUart) nextToken() []byte {
	tok := make([]byte, 4)
	t.rand.Read(tok)
	return tok
}

var methodToCodeTable = map[string]coapmsg.COAPCode{
	"GET":    coapmsg.GET,
	"POST":   coapmsg.POST,
	"PUT":    coapmsg.PUT,
	"DELETE": coapmsg.DELETE,
}

// methodToCode returns the code for a given CoAP method.
// Default is GET, use ValidMethod to ensure a valid method.
func methodToCode(method string) coapmsg.COAPCode {
	if code, ok := methodToCodeTable[method]; ok {
		return code
	}
	return coapmsg.GET
}

// Last successful "any" port. Will be tried first before iterating
var lastAny = ""

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

func isWindows() bool {
	return runtime.GOOS == "windows"
}
