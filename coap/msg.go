package coap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

type Message struct {
	Version    uint8
	Type       MsgType
	Code       MsgCode
	Token      []byte
	MessageID  uint16
	Options    Options
	Payload    []byte
}

func NewMessage() *Message {
	msg := new(Message)
	
	msg.Version = 1
	msg.Type = RST
	msg.Code = EMPTY
	msg.MessageID = 0
	msg.Payload = nil
	msg.Options = nil
	msg.Token = nil
	
	return msg
}

func NewMessageParse(rawBytes []byte) (*Message, error) {
	msg := new(Message)
	
	if err:=msg.Parse(rawBytes); err != nil {
		return nil, err
	}
	
	return msg, nil
}



func (msg *Message) String() string {
	var buffer bytes.Buffer
	
	buffer.WriteString("+++++++++++++++++++++++\r\n")
	

	buffer.WriteString(fmt.Sprintf("-Type: %s (0x%x)\r\n", MsgTypeName[msg.Type], msg.Type)); 
	buffer.WriteString(fmt.Sprintf("-Code: %s (0x%x)\r\n", MsgCodeName[msg.Code], msg.Code));
	
	buffer.WriteString(fmt.Sprintf("-Token [%d byte]: ", len(msg.Token))); 
	for _,v := range(msg.Token) {
		buffer.WriteString(fmt.Sprintf("0x%x(%c) ",v,v))
	}
	buffer.WriteString("\r\n")
	
	buffer.WriteString(fmt.Sprintf("-MessageID: %d | 0x%x\r\n", msg.MessageID, msg.MessageID)); 
	
	buffer.WriteString(fmt.Sprintf("-Options [num = %d]:\r\n", len(msg.Options))); 
	for _,o := range(msg.Options) {
		buffer.WriteString(fmt.Sprintf("-->Option #%d (Length=%d) :",o.Number,o.Len()))
		
		for _,b := range(o.Value){
			buffer.WriteString(fmt.Sprintf("%c",b))
		}
	
		buffer.WriteString(" | ")
		
		for _,b := range(o.Value){
			buffer.WriteString(fmt.Sprintf("0x%x ",b))
		}
		buffer.WriteString("\r\n")
		
	}
	
	if len(msg.Payload) > 0 {
		buffer.WriteString(fmt.Sprintf("-Payload [%d byte]: \r\n-->", len(msg.Payload)))
			for _,b := range(msg.Payload){
				buffer.WriteString(fmt.Sprintf("%c",b))
		}
		
		buffer.WriteString(" | ")
		for _,b := range(msg.Payload){
				buffer.WriteString(fmt.Sprintf("0x%x ",b))
		}
	} else  {
		buffer.WriteString("-Payload: none")
	}

	buffer.WriteString("\r\n+++++++++++++++++++++++\r\n")
	return buffer.String()
}


type MsgType uint8

const COAP_VERSION_1_0 byte = 1

const (
	CON MsgType = 0
	NON               = 1
	ACK               = 2
	RST               = 3
)

var MsgTypeName = [256]string{
	CON: "CON",
	NON: "NON",
	ACK: "ACK",
	RST: "RST",
}

func init() {
	for i := range MsgTypeName {
		if MsgTypeName[i] == "" {
			MsgTypeName[i] = fmt.Sprintf("Unknown (0x%x)", i)
		}
	}
}

type MsgCode uint8

const (
	EMPTY                                MsgCode = (0 << 5) | 0
	REQ_GET                                            = (0 << 5) | 1
	REQ_POST                                           = (0 << 5) | 2
	REQ_PUT                                            = (0 << 5) | 3
	REQ_DELETE                                         = (0 << 5) | 4
	RESP_SUCCESS_CREATED_2_01                          = (2 << 5) | 1 //only used on response to "POST" and "PUT" like HTTP 201
	RESP_SUCCESS_DELETED_2_02                          = (2 << 5) | 2 //only used on response to "DELETE" and "POST" like HTTP 204
	RESP_SUCCESS_VALID_2_03                            = (2 << 5) | 3
	RESP_SUCCESS_CHANGED_2_04                          = (2 << 5) | 4 //only used on response to "POST" and "PUT" like HTTP 204
	RESP_SUCCESS_CONTENT_2_05                          = (2 << 5) | 5 //only used on response to "GET" like HTTP 200 (OK)
	RESP_ERROR_BAD_REQUEST_4_00                        = (4 << 5) | 0 //like HTTP 400 (OK)
	RESP_ERROR_UNAUTHORIZED_4_01                       = (4 << 5) | 1
	RESP_BAD_OPTION_4_02                               = (4 << 5) | 2
	RESP_FORBIDDEN_4_03                                = (4 << 5) | 3
	RESP_NOT_FOUND_4_04                                = (4 << 5) | 4
	RESP_METHOD_NOT_ALLOWED_4_05                       = (4 << 5) | 5
	RESP_METHOD_NOT_ACCEPTABLE_4_06                    = (4 << 5) | 6
	RESP_PRECONDITION_FAILED_4_12                      = (4 << 5) | 12
	RESP_REQUEST_ENTITY_TOO_LARGE_4_13                 = (4 << 5) | 13
	RESP_UNSUPPORTED_CONTENT_FORMAT_4_15               = (4 << 5) | 15
	RESP_INTERNAL_SERVER_ERROR_5_00                    = (5 << 5) | 0
	RESP_NOT_IMPLEMENTED_5_01                          = (5 << 5) | 1
	RESP_BAD_GATEWAY_5_02                              = (5 << 5) | 2
	RESP_SERVICE_UNAVAILABLE_5_03                      = (5 << 5) | 3
	RESP_GATEWAY_TIMEOUT_5_04                          = (5 << 5) | 4
	RESP_PROXYING_NOT_SUPPORTED_5_05                   = (5 << 5) | 5
)

var MsgCodeName = [256]string{
	EMPTY:                                "EMPTY",
	REQ_GET:                              "REQ_GET",
	REQ_POST:                             "REQ_POST",
	REQ_PUT:                              "REQ_PUT",
	REQ_DELETE:                           "REQ_DELETE",
	RESP_SUCCESS_CREATED_2_01:            "RESP_SUCCESS_CREATED_2_01",
	RESP_SUCCESS_DELETED_2_02:            "RESP_SUCCESS_DELETED_2_02",
	RESP_SUCCESS_VALID_2_03:              "RESP_SUCCESS_VALID_2_03",
	RESP_SUCCESS_CHANGED_2_04:            "RESP_SUCCESS_CHANGED_2_04",
	RESP_SUCCESS_CONTENT_2_05:            "RESP_SUCCESS_CONTENT_2_05",
	RESP_ERROR_BAD_REQUEST_4_00:          "RESP_ERROR_BAD_REQUEST_4_00",
	RESP_ERROR_UNAUTHORIZED_4_01:         "RESP_ERROR_UNAUTHORIZED_4_01",
	RESP_BAD_OPTION_4_02:                 "RESP_BAD_OPTION_4_02",
	RESP_FORBIDDEN_4_03:                  "RESP_FORBIDDEN_4_03",
	RESP_NOT_FOUND_4_04:                  "RESP_NOT_FOUND_4_04",
	RESP_METHOD_NOT_ALLOWED_4_05:         "RESP_METHOD_NOT_ALLOWED_4_05",
	RESP_METHOD_NOT_ACCEPTABLE_4_06:      "RESP_METHOD_NOT_ACCEPTABLE_4_06",
	RESP_PRECONDITION_FAILED_4_12:        "RESP_PRECONDITION_FAILED_4_12",
	RESP_REQUEST_ENTITY_TOO_LARGE_4_13:   "RESP_REQUEST_ENTITY_TOO_LARGE_4_13",
	RESP_UNSUPPORTED_CONTENT_FORMAT_4_15: "RESP_UNSUPPORTED_CONTENT_FORMAT_4_15",
	RESP_INTERNAL_SERVER_ERROR_5_00:      "RESP_INTERNAL_SERVER_ERROR_5_00",
	RESP_NOT_IMPLEMENTED_5_01:            "RESP_NOT_IMPLEMENTED_5_01",
	RESP_BAD_GATEWAY_5_02:                "RESP_BAD_GATEWAY_5_02",
	RESP_SERVICE_UNAVAILABLE_5_03:        "RESP_SERVICE_UNAVAILABLE_5_03",
	RESP_GATEWAY_TIMEOUT_5_04:            "RESP_GATEWAY_TIMEOUT_5_04",
	RESP_PROXYING_NOT_SUPPORTED_5_05:     "RESP_PROXYING_NOT_SUPPORTED_5_05",
}

func init() {
	for i := range MsgCodeName {
		if MsgCodeName[i] == "" {
			MsgCodeName[i] = fmt.Sprintf("Unknown (0x%x)", i)
		}
	}
}


func (msg Message) IsRequest() bool {
	if msg.Code == REQ_GET || msg.Code == REQ_POST || msg.Code == REQ_PUT || msg.Code == REQ_DELETE {
		return true
	}
	
	return false
}

func (msg Message) RespType() MsgType {
	
	if msg.Type == CON {
		return ACK
	}else {
		return NON
	}
}


func (msg *Message) ResetToEmpty(Mid uint16) {
	msg.Version = 1
	msg.Type = RST
	msg.Code = EMPTY
	msg.MessageID = Mid
	msg.Payload = nil
	msg.Options = nil
	msg.Token = nil
}

func (msg *Message) Parse(srcArr []byte) error {

	if len(srcArr) < 4 {
		return errors.New("COAP_PARSE_DATAGRAM_TOO_SHORT")
	}

	msg.ResetToEmpty(0)

	//1st Header Byte
	msg.Version = srcArr[0] >> 6
	if msg.Version != 1 {
		return errors.New("COAP_PARSE_UNKOWN_COAP_VERSION")
	}

	msg.Type = MsgType((srcArr[0] & 0x30) >> 4)
	tokenLen := uint8(srcArr[0] & 0xf)

	if tokenLen > 8 {
		return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR")
	}

	//2nd & 3rd Header Byte
	msg.Code = MsgCode(srcArr[1])

	if msg.Code == EMPTY && (tokenLen != 0 || len(srcArr) != 4) {
		return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR")
	}

	codeClass := uint8(msg.Code) >> 5
	if codeClass == 1 || codeClass == 6 || codeClass == 7 {
		return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR")
	}

	//4th Header Byte
	msg.MessageID = binary.BigEndian.Uint16(srcArr[2:4])

	//further parsing locations depend on parsed 4Byte CoAP Header -> use of offset addressing
	offset := 4

	if len(srcArr) == offset {
		return nil
	}

	//Tokens (if any)
	msg.Token = make([]byte, tokenLen)
	copy(msg.Token, srcArr[offset:offset+int(tokenLen)])

	offset += int(tokenLen)

	if len(srcArr) == offset {
		return nil
	}

	//Options (if any)
	msg.parseOptions(srcArr[offset:]) //+appends payload to message (if any)

	return nil
}

func (msg *Message) parseOptions(srcArr []byte) error {
	//srcArr points to the beginning of Option section @ raw datagram byte array
	//length includes payload marker & payload (if any)

	lastOptionNumber := 0
	offset := 0
	srcLength := len(srcArr)

	for offset < srcLength {
		if srcArr[offset] == 0xff {

			if srcLength-offset < 2 {
				return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR") //at least one byte payload must follow to the payload marker
			}
			msg.Payload = make([]byte, srcLength - (offset + 1))
			copy(msg.Payload, srcArr[offset+1:]) //everything after marker is payload
			return nil

		} else {

			currOptDeltaField := int(srcArr[offset] >> 4)
			currOptDelta := currOptDeltaField //init with field data, but can be overwritten if field set to 13 or 14
			currOptLengthField := int(srcArr[offset] & 0x0f)
			currOptLength := currOptLengthField //init with field data, but can be overwritten if field set to 13 or 14

			offset++

			//Option Delta extended (if any)
			if currOptDeltaField == 13 {

				currOptDelta = int(srcArr[offset] + 13)
				offset++
			} else if currOptDeltaField == 14 {

				//currOptDelta = ((( uint16(srcArr[offset]) << 8)  |  ((uint16(srcArr[offset+1]))) + 269
				currOptDelta = ((int(srcArr[offset]) << 8) | int(srcArr[offset+1])) + 269
				offset += 2
			} else if currOptDeltaField == 15 {
				return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR")
			}

			//Option Length extended (if any)
			if currOptLengthField == 13 {

				currOptLength = int(srcArr[offset]) + 13
				offset++
			} else if currOptLengthField == 14 {

				currOptLength = ((int(srcArr[offset]) << 8) | int(srcArr[offset+1])) + 269
				offset += 2
			} else if currOptLengthField == 15 {
				return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR")
			}

			if currOptLength > 1023 {

				return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR")
			}
			if (srcLength - offset) < currOptLength {
				return errors.New("COAP_PARSE_MESSAGE_FORMAT_ERROR")
			}

			lastOptionNumber = currOptDelta + lastOptionNumber

			optVal := make([]byte, currOptLength)
			copy(optVal, srcArr[offset:offset+currOptLength])
			msg.Options = append(msg.Options, Option{Number: uint16(lastOptionNumber),  Value: optVal})

			offset = offset + currOptLength
		}

	}

	return nil
}


func (msg *Message) Bytes() ([]byte, error) {

	buf := bytes.Buffer{}

	if msg.Code == EMPTY {
		msg.Payload = nil
		msg.Token = nil
	}

	var bt byte = 0

	// 4Byte Header (see p.16 RFC7252)
	bt |= (COAP_VERSION_1_0 & 3) << 6
	bt |= (byte(msg.Type) & 3) << 4
	bt |= (byte(len(msg.Token)) & 15)
	buf.WriteByte(bt) //No°1
	bt = byte(msg.Code)
	buf.WriteByte(bt) //No°2
	bt = byte(msg.MessageID >> 8)
	buf.WriteByte(bt) //No°3
	bt = byte(msg.MessageID & 0xff)
	buf.WriteByte(bt) //No°4

	// Token (0 to 8 Bytes)
	for i := 0; i < len(msg.Token); i++ {
		buf.WriteByte(msg.Token[i])
	}

	// Options
	optbuf, _ := msg.optionsToBytes()
	buf.Write(optbuf)

	//Payload
	if len(msg.Payload) > 0 {
		buf.WriteByte(0xff) //marker
		buf.Write(msg.Payload)
	}

	return buf.Bytes(), nil
}

func (msg *Message) optionsToBytes() ([]byte, error) {

	sort.Stable(msg.Options)

	optHeader := make([]byte, 1, 5)
	lastOptNumber := uint16(0)
	currDelta := uint16(0) //current Delta to privious option

	buf := bytes.Buffer{}

	for _, opt := range msg.Options {
		//Inits for Option Packing
		currDelta = opt.Number - lastOptNumber
		lastOptNumber = opt.Number

		optHeader = optHeader[:1]
		optHeader[0] = 0
		//Delta Bytes
		if currDelta < 13 {
			optHeader[0] |= byte(currDelta) << 4
		} else if currDelta < 269 {
			optHeader = optHeader[:2]
			optHeader[0] |= (13 << 4)
			optHeader[1] = uint8(currDelta) - 13
		} else {
			optHeader = optHeader[:3]
			optHeader[0] |= uint8(14) << 4
			optHeader[1] = byte(currDelta-269) >> 8
			optHeader[2] = byte(currDelta-269) & 0xff
		}

		//Length Bytes
		
		optLen := opt.Len()
		
		if optLen < 13 {
			optHeader[0] |= uint8(optLen)
		} else if optLen < 269 {
			optHeader = optHeader[:len(optHeader)+1]
			optHeader[0] |= byte(13)
			optHeader[len(optHeader)-1] = byte(optLen) - 13
		} else {
			optHeader = optHeader[:len(optHeader)+2]
			optHeader[0] |= byte(14)
			optHeader[len(optHeader)-2] = byte(optLen-269) >> 8
			optHeader[len(optHeader)-1] = byte(optLen-269) & 0xff
		}

		buf.Write(optHeader)

		//Option Values
		buf.Write(opt.Value)
	}

	return buf.Bytes(), nil
}
