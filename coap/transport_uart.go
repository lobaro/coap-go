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
	"strconv"
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

// Timeout to close a serial com port when no data is received
const CONNECTION_TIMEOUT = 1 * time.Minute

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

	// UART parameters. In future we might want to configure this per port.
	Baud        int           // BaudRate
	ReadTimeout time.Duration // Total timeout
	Size        byte          // Size is the number of data bits. If 0, DefaultSize is used.
	Parity      Parity        // Parity is the bit to use and defaults to ParityNone (no parity bit).
	StopBits    StopBits      // Number of stop bits to use. Default is 1 (1 stop bit).

	connections []Connection
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

func logMsg(msg *coapmsg.Message, info string) {
	bin, _ := msg.MarshalBinary()

	options := logrus.Fields{}
	for _, o := range msg.OptionsRaw() {
		options["opt:"+strconv.Itoa(int(o.ID))] = o.ToBytes()
	}

	logrus.WithField("Code", msg.Code.String()).
		WithField("Type", msg.Type.String()).
		WithField("Token", msg.Token).
		WithField("MessageID", msg.MessageID).
		WithField("Payload", msg.Payload).
		WithField("OptionCount", msg.OptionsRaw().Len()).
		WithFields(options).
		WithField("Bin", bin).
		Info("CoAP message: " + info)
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

	if req.URL == nil {
		return nil, errors.New(fmt.Sprint("coap: Missing request URL"))
	}
	if req.URL.Scheme != UartScheme {
		return nil, errors.New(fmt.Sprint("coap: Invalid URL scheme, expected "+UartScheme+" but got: ", req.URL.Scheme))
	}

	conn, err := t.connect(req.URL.Host)
	if err != nil {
		return
	}

	if err := sendMessage(conn.writer, reqMsg); err != nil {
		return nil, err
	}

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
		//    |   ACK [0x7a10]   |
		//    |<-----------------+ <- We are here!
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
		if resMsg.Type == coapmsg.Confirmable {
			ack := coapmsg.NewAck(resMsg.MessageID)
			if err := sendMessage(conn.writer, &ack); err != nil {
				return nil, err
			}
		}
	}

	if !bytesAreEqual(reqMsg.Token, resMsg.Token) {
		return nil, errors.New(fmt.Sprintf("coap: Token of response does not match %x != %x", reqMsg.Token, resMsg.Token))
	}

	res = buildResponse(req, resMsg)

	// Handle observe
	if req.Options.Get(coapmsg.Observe) == 0 {
		go waitForNotify(conn, req, res)
	}

	return res, nil
}

func waitForNotify(conn Connection, req *Request, currResponse *Response) {
	currResponse.Next = make(chan *Response, 1)
	defer close(currResponse.Next)

	resMsg, err := readResponse(req.Context(), conn)
	if err != nil {
		logrus.WithError(err).Error("Failed to read notify response")
		return
	}
	if resMsg.Type == coapmsg.Confirmable {
		ack := coapmsg.NewAck(resMsg.MessageID)
		if err := sendMessage(conn, &ack); err != nil {
			logrus.WithError(err).Error("Failed to send ACK for notify")
			return
		}
	}
	res := buildResponse(req, resMsg)
	// TODO: What does the server send to tell us to stop listening?

	currResponse.Next <- res
	select {
	case <-req.Context().Done():
		logrus.Info("Stopped observer, request context timed out!")
		return
	default:
		go waitForNotify(conn, req, res)
	}

	return
}

func buildResponse(req *Request, resMsg *coapmsg.Message) *Response {
	return &Response{
		StatusCode: int(resMsg.Code),
		Status:     fmt.Sprintf("%d.%02d %s", resMsg.Code.Class(), resMsg.Code.Detail(), resMsg.Code.String()),
		Body:       ioutil.NopCloser(bytes.NewReader(resMsg.Payload)),
		Request:    req,
	}
}

func sendMessage(writer CoapPacketWriter, msg *coapmsg.Message) error {
	bin, err := msg.MarshalBinary()
	if err != nil {
		return err
	}

	logMsg(msg, "Send")
	err = writer.WritePacket(bin)
	if err != nil {
		return err
	}
	return nil
}

func readResponse(ctx context.Context, conn Connection) (*coapmsg.Message, error) {
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
	case err := <-errorCh:
		return nil, err
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
	logMsg(&msg, "Received")

	return &msg, nil
}

func readPacket(slipReader CoapPacketReader) ([]byte, error) {
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
		if q != "" {
			msg.SetOption(coapmsg.URIQuery, q)
		}
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

func (t *TransportUart) connect(host string) (*serialConnection, error) {
	serialCfg := t.newSerialConfig()
	if host == "any" {
		serialCfg.Name = host
	} else if !isWindows() {
		serialCfg.Name = "/dev/" + host
	} else {
		serialCfg.Name = host
	}

	// can recycle connection?
	for i, c := range t.connections {
		if c.Closed() {
			t.connections = deleteConnection(t.connections, i)
			continue
		}

		if c, ok := c.(*serialConnection); (ok && c.config.Name == serialCfg.Name) || serialCfg.Name == "any" {
			logrus.WithField("Port", c.config.Name).Info("Reuseing Serial Port")
			return c, nil
		}
	}

	port, err := openComPort(serialCfg)
	if err != nil {
		return nil, err
	}
	conn := &serialConnection{
		config:   serialCfg,
		port:     port,
		reader:   slip.NewReader(port),
		writer:   slip.NewWriter(port),
		deadline: time.Now().Add(CONNECTION_TIMEOUT),
	}
	t.connections = append(t.connections, conn)
	go conn.closeAfterDeadline()
	return conn, nil
}
