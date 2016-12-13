package rs232

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Lobaro/coap-go/coap"
	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/Lobaro/slip"
	"github.com/tarm/serial"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type Transport struct {
	mu        *sync.Mutex
	lastMsgId uint16     // Sequence counter
	rand      *rand.Rand // Random source for token generation
	// TODO: add parameter for serial connection like Baud rate.
}

func NewTransport() *Transport {
	return &Transport{
		mu:   &sync.Mutex{},
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

}

func (t *Transport) RoundTrip(req *coap.Request) (res *coap.Response, err error) {

	if req == nil {
		return nil, errors.New("coap: Got nil request")
	}

	if !req.Confirmable {
		// TODO: Implement non-confirmable requests
		// This will need some concept of "interactions" matched via Message Ids and Tokens
		return nil, errors.New("coap: Confirmable request not stupported yet!")
	}

	reqMsg, err := t.buildMessage(req)
	if err != nil {
		return
	}

	//###########################################
	// Open the connection and write the request
	//###########################################

	serialCfg := newSerialConfig()

	if req.URL == nil {
		return nil, errors.New(fmt.Sprint("coap: Missing request URL"))
	}
	if req.URL.Scheme != "coap" {
		return nil, errors.New(fmt.Sprint("coap: Invalid URL scheme: ", req.URL.Scheme))
	}

	serialCfg.Name = req.URL.Host

	conn, err := openComPort(serialCfg)
	defer conn.Close()
	if err != nil {
		return
	}

	bin, err := reqMsg.MarshalBinary()
	if err != nil {
		return
	}

	slipWriter := slip.NewWriter(conn)
	slipWriter.WritePacket(bin)

	//###########################################
	// Read the response
	//###########################################
	slipReader := slip.NewReader(conn)

	buf := &bytes.Buffer{}

	var isPrefix bool

	// TODO: Implement read timeouts and retries until first ACK is received
	for {
		var p []byte
		p, isPrefix, err = slipReader.ReadPacket()
		buf.Write(p)

		if err != nil {
			break
		}

		if !isPrefix {
			break
		}
	}

	if err != nil {
		return nil, err
	}
	if isPrefix {
		return nil, errors.New("coap: Did read incomplete response")
	}

	// Did we got an ACK or response?
	resMsg, err := coapmsg.ParseMessage(buf.Bytes())
	if err != nil {
		return
	}

	if req.Confirmable && resMsg.MessageID != reqMsg.MessageID {
		return nil, errors.New("coap: MessageId of response does not match")
	}

	if !bytesAreEqual(reqMsg.Token, resMsg.Token) {
		return nil, errors.New("coap: Token of response does not match")
	}

	// TODO: Handle Types: Con and Reset correctly - now we just don't care

	if resMsg.Type == coapmsg.Acknowledgement && resMsg.Code == coapmsg.Empty {
		// TODO: Implement delayed responses
		//  Client              Server
		//    |                  |
		//    |   CON [0x7a10]   |
		//    | GET /temperature |
		//    |   (Token 0x73)   |
		//    +----------------->|
		//    |                  |
		//    |   ACK [0x7a10]   |
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
		return nil, errors.New("coap: Received empty ACK. Delayed responses not supported yet!")
	}

	res = &coap.Response{
		StatusCode: int(resMsg.Code),
		Status:     fmt.Sprintf("%d.%d %s", resMsg.Code.Class(), resMsg.Code.Detail(), resMsg.Code.String()),
		Body:       ioutil.NopCloser(bytes.NewReader(resMsg.Payload)),
		Request:    req,
	}
	return res, nil
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

func newSerialConfig() *serial.Config {
	return &serial.Config{
		Name:        "",
		Baud:        115200,
		Parity:      serial.ParityNone,
		Size:        0,
		ReadTimeout: time.Millisecond * 500,
		StopBits:    serial.Stop1,
	}
}

// BuildMessage creates a coap message based on the request
// Takes care of closing the request body
func (t *Transport) buildMessage(req *coap.Request) (*coapmsg.Message, error) {
	defer req.Body.Close()
	if !coap.ValidMethod(req.Method) {
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

func (t *Transport) nextMessageId() uint16 {
	t.mu.Lock()
	t.lastMsgId++
	msgId := t.lastMsgId
	t.mu.Unlock()
	return msgId
}

func (t *Transport) nextToken() []byte {
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

func openComPort(serialCfg *serial.Config) (port *serial.Port, err error) {
	if serialCfg.Name == "" {
		for i := 0; i < 99; i++ {
			serialCfg.Name = fmt.Sprintf("COM%d", i)
			//logrus.WithField("port", serialCfg.Name).Info("Try to open COM port")
			port, err = serial.OpenPort(serialCfg)
			if err == nil {
				return
			}

		}
		err = errors.New(fmt.Sprint("coap: No open COM ports: ", err.Error()))
		return
	}
	port, err = serial.OpenPort(serialCfg)
	return
}
