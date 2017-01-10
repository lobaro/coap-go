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
const UART_CONNECTION_TIMEOUT = 1 * time.Minute

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
	mu           *sync.Mutex
	lastMsgId    uint16     // Sequence counter
	lastTokenSeq uint8      // Sequence counter
	rand         *rand.Rand // Random source for token generation

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

func msgLogEntry(msg *coapmsg.Message) *logrus.Entry {
	//bin, _ := msg.MarshalBinary()

	options := logrus.Fields{}
	for _, o := range msg.OptionsRaw() {
		options["opt:"+strconv.Itoa(int(o.ID))] = o.ToBytes()
	}

	return log.WithField("Code", msg.Code.String()).
		WithField("Type", msg.Type.String()).
		WithField("Token", msg.Token).
		WithField("MessageID", msg.MessageID).
		//WithField("Payload", msg.Payload).
		WithField("OptionCount", msg.OptionsRaw().Len()).
		WithFields(options)
	//WithField("Bin", bin)
}

func logMsg(msg *coapmsg.Message, info string) {
	msgLogEntry(msg).Info("CoAP message: " + info)
}

func (t *TransportUart) RoundTrip(req *Request) (res *Response, err error) {

	if req == nil {
		return nil, errors.New("coap: Got nil request")
	}

	// The client might set a specific token, e.g. to cancel an observe.
	// If there is no token set we create a random token.
	if len(req.Token) == 0 {
		req.Token = t.nextToken()
	}

	reqMsg, err := t.buildReqMessage(req)
	if err != nil {
		return
	}

	//###########################################
	// Open the connection
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

	//###########################################
	// Start an interaction and send the request
	//###########################################

	// When canceling an observer we must reuse the interaction
	ia, err := conn.FindInteraction(req.Token, MessageId(0))
	if ia == nil || err != nil {
		// TODO: Pass t.NextToken instead of reqMsg and set the token on sendMessage?
		ia = t.startInteraction(conn, reqMsg)
	} else {
		// A new round trip on an existing interaction can only work when we are not listening
		// for notifications. Else the notifications eat up all responses from the server.
		ia.StopListenForNotifications()
	}

	resMsg, err := ia.RoundTrip(req.Context(), reqMsg)
	if err != nil {
		return nil, wrapError(err, fmt.Sprint("Failed Interaction Roundtrip with Token ", ia.token))
	}

	//###########################################
	// Build and return the response
	//###########################################

	res = buildResponse(req, resMsg)

	go waitForNotify(ia, req, res)

	return res, nil
}

func (t *TransportUart) startInteraction(conn Connection, reqMsg *coapmsg.Message) *Interaction {
	ia := &Interaction{
		token:     Token(reqMsg.Token),
		conn:      conn,
		receiveCh: make(chan *coapmsg.Message, 0),
	}

	log.WithField("Token", Token(reqMsg.Token)).Info("Start interaction")

	conn.AddInteraction(ia)

	return ia
}

func waitForNotify(ia *Interaction, req *Request, currResponse *Response) {

	defer close(currResponse.next)

	select {
	case resMsg, ok := <-ia.NotificationCh:
		if ok {
			res := buildResponse(req, resMsg)
			currResponse.next <- res

			go waitForNotify(ia, req, res)
		} else {
			// Also happens for all non observe requests since ia.NotificationCh will be closed.
			log.Info("Stopped observer, no more notifies expected.")
		}
	}
}

func buildResponse(req *Request, resMsg *coapmsg.Message) *Response {
	nextCh := make(chan *Response, 0)
	return &Response{
		StatusCode: int(resMsg.Code),
		Status:     fmt.Sprintf("%d.%02d %s", resMsg.Code.Class(), resMsg.Code.Detail(), resMsg.Code.String()),
		Body:       ioutil.NopCloser(bytes.NewReader(resMsg.Payload)),
		Request:    req,
		next:       nextCh,
	}
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
func (t *TransportUart) buildReqMessage(req *Request) (*coapmsg.Message, error) {
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
		Token:     req.Token,
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
	defer t.mu.Unlock()
	t.lastMsgId++
	msgId := t.lastMsgId
	return msgId
}

func (t *TransportUart) nextToken() []byte {
	// It's critical to not get the same token twice,
	// since we identify our interactions by the token
	t.mu.Lock()
	defer t.mu.Unlock()
	tok := make([]byte, 4)
	t.rand.Read(tok)
	t.lastTokenSeq++
	tok[0] = t.lastTokenSeq
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

var connectMutex sync.Mutex

func (t *TransportUart) connect(host string) (*serialConnection, error) {
	connectMutex.Lock()
	defer connectMutex.Unlock()

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
			log.WithField("Port", c.config.Name).Info("Reuseing Serial Port")
			return c, nil
		}
	}

	port, err := openComPort(serialCfg)
	if err != nil {
		return nil, wrapError(err, "Failed to open serial port")
	}
	conn := &serialConnection{
		config:   serialCfg,
		port:     port,
		reader:   slip.NewReader(port),
		writer:   slip.NewWriter(port),
		deadline: time.Now().Add(UART_CONNECTION_TIMEOUT),
	}
	t.connections = append(t.connections, conn)
	go conn.closeAfterDeadline()
	go conn.StartReceiveLoop(context.Background())
	return conn, nil
}
