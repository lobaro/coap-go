package liblobarocoap

/*
#cgo LDFLAGS: "-LC:/dev/cpath/github.com/Lobaro/lobaro-coap" -L${SRCDIR} -llobaro_coap
#include "liblobaro_coap.h"
#include "coap_options.h"
#include <stdio.h>
#include <stdlib.h>

uint32_t go_rtc1HzCnt();
bool go_Tx(SocketHandle_t socketHandle, NetPacket_t* pPacket);
void go_debugPuts(char *s);
void go_debugPutc(char c);
CoAP_HandlerResult_t go_ResourceHandler(CoAP_Message_t* pReq, CoAP_Message_t* pResp);

//static inline void go_debugPuts(char *s) {
//	printf("%s", s);
//	fflush(stdout);
//}

static inline CoAP_Socket_t* CreateSocket(SocketHandle_t handle) {
	CoAP_Socket_t* pSocket = CoAP_NewSocket(handle);
	pSocket->Tx = go_Tx;
	return pSocket;
}

*/
import "C"
import (
	"encoding/binary"
	"fmt"
	"github.com/Lobaro/coap-go/coapmsg"
	"github.com/Sirupsen/logrus"
	"log"
	"net"
	"time"
	"unsafe"
)

var currentHandle uintptr = 0

var resources = make(map[string]*Resource)

type HandlerResult byte

const (
	OK       = HandlerResult(0)
	POSTPONE = HandlerResult(1)
	ERROR    = HandlerResult(2)
)

type Resource struct {
	ref     unsafe.Pointer // type *C.struct_CoAP_Res
	Handler func(req coapmsg.Message, res *coapmsg.Message) HandlerResult
}

type Socket struct {
	Handle uintptr
}

type Packet struct {
	Data     []byte
	RemoteEp CoapEndpoint
	Socket   Socket
}

type EndpointType int

const (
	EP_IPV4 = EndpointType(0)
	EP_IPV6 = EndpointType(1)
	EP_UART = EndpointType(2)
	EP_BTLE = EndpointType(3)
)

type CoapEndpoint struct {
	Type    EndpointType
	Ip      net.IP
	Port    int
	ComPort byte
}

var PendingResponses = make(chan Packet, 50)

var coapMemory = make([]byte, 4096)

func init() {
	log.Println("Allocated go memory at", unsafe.Pointer(&coapMemory))

	api := C.CoAP_API_t{}

	api.debugPuts = (*[0]byte)(unsafe.Pointer(C.go_debugPuts))
	api.rtc1HzCnt = (*[0]byte)(unsafe.Pointer(C.go_rtc1HzCnt))
	cfg := C.CoAP_Config_t{}

	cfg.Memory = (*C.uint8_t)(unsafe.Pointer(&coapMemory))
	cfg.MemorySize = C.int16_t(len(coapMemory))

	C.CoAP_Init(api, cfg)

	go func() {
		for {
			DoWork()
			<-time.After(100 * time.Millisecond)
		}
	}()
}

func NewSocket() Socket {
	currentHandle++
	cSocket := C.CreateSocket(unsafe.Pointer(currentHandle))

	return Socket{Handle: uintptr((cSocket.Handle))}
}

func HandleIncomingIPv4Packet(socket Socket, senderIp net.IP, senderPort int, data []byte) {
	cData := C.CBytes(data)
	defer C.free(cData)

	cPacket := C.NetPacket_t{}
	cPacket.size = C.uint16_t(len(data))
	cPacket.pData = (*C.uint8_t)(cData)
	cPacket.remoteEp = createIpv4Ep(senderIp, senderPort)
	// TODO: Add metadata?

	C.CoAP_HandleIncomingPacket(unsafe.Pointer(socket.Handle), &cPacket)
}

func HandleIncomingUartPacket(socket Socket, senderPort byte, data []byte) {
	cData := C.CBytes(data)
	defer C.free(cData)

	cPacket := C.NetPacket_t{}
	cPacket.size = C.uint16_t(len(data))
	cPacket.pData = (*C.uint8_t)(cData)
	cPacket.remoteEp = createUartEp(senderPort)
	// TODO: Add metadata?

	C.CoAP_HandleIncomingPacket(unsafe.Pointer(socket.Handle), &cPacket)
}

func DoWork() {
	C.CoAP_doWork()
}

// CoAP_Res_t* CoAP_CreateResource(char* Uri, char* Descr,CoAP_ResOpts_t Options, CoAP_ResourceHandler_fPtr_t pHandlerFkt, CoAP_ResourceNotifier_fPtr_t pNotifierFkt );
// typedef CoAP_HandlerResult_t (*CoAP_ResourceHandler_fPtr_t)(CoAP_Message_t* pReq, CoAP_Message_t* pResp);
// typedef CoAP_HandlerResult_t (*CoAP_ResourceNotifier_fPtr_t)(CoAP_Observer_t* pListObservers, CoAP_Message_t* pResp);
func CreateResource(uri string, description string, allowedMethods ...coapmsg.COAPCode) *Resource {
	opts := C.CoAP_ResOpts_t{}

	for _, m := range allowedMethods {
		opts.AllowedMethods |= 1 << m.Detail()
	}
	resourceHandler := (*[0]byte)(unsafe.Pointer(C.go_ResourceHandler))
	res := C.CoAP_CreateResource(C.CString(uri), C.CString(description), opts, resourceHandler, nil)

	if res == nil {
		return nil
	}

	msg := coapmsg.Message{}
	msg.SetPathString(uri)

	resource := &Resource{
		ref: unsafe.Pointer(res),
	}

	resources[msg.PathString()] = resource

	return resource
}

//export go_rtc1HzCnt
func go_rtc1HzCnt() C.uint32_t {
	return C.uint32_t(time.Now().Unix())
}

//export go_Tx
func go_Tx(socketHandle C.SocketHandle_t, pPacket *C.NetPacket_t) C.bool {
	logrus.Info("Sending CoAP packet to remote")

	msgBytes := C.GoBytes(unsafe.Pointer(pPacket.pData), C.int(pPacket.size))
	PendingResponses <- Packet{
		Data:     msgBytes,
		RemoteEp: coapEndpoint(pPacket.remoteEp),
		Socket:   Socket{Handle: uintptr(socketHandle)},
	}

	return C.bool(true)
}

//void go_debugPuts(char *s);
//export go_debugPuts
func go_debugPuts(s *C.char) {
	fmt.Print(C.GoString(s))
}

func toGoMessage(cMsg *C.CoAP_Message_t) coapmsg.Message {
	msg := coapmsg.Message{}
	msg.Payload = C.GoBytes(unsafe.Pointer(cMsg.Payload), C.int(cMsg.PayloadLength))
	msg.Type = coapmsg.COAPType(cMsg.Type)
	msg.Code = coapmsg.COAPCode(cMsg.Code)
	msg.MessageID = uint16(cMsg.MessageID)
	msg.Token = make([]byte, 8)
	binary.LittleEndian.PutUint64(msg.Token, uint64(cMsg.Token64))

	for opt := cMsg.pOptionsList; opt != nil; opt = opt.next {
		msg.AddOptionFromBytes(coapmsg.OptionID(opt.Number), C.GoBytes(unsafe.Pointer(opt.Value), C.int(opt.Length)))
	}
	return msg
}

func toCMessage(goMsg coapmsg.Message, cMsg *C.CoAP_Message_t) {
	payload := C.CBytes(goMsg.Payload)
	cMsg.Payload = (*C.uint8_t)(payload)
	// TODO: When can we actually free?
	go func() {
		select {
		case <-time.After(5 * time.Second):
			C.free(payload)
		}
	}()
	cMsg.PayloadLength = C.uint16_t(len(goMsg.Payload))
	cMsg.Type = C.CoAP_MessageType_t(goMsg.Type)
	cMsg.Code = C.CoAP_MessageCode_t(goMsg.Code)
	cMsg.MessageID = C.uint16_t(goMsg.MessageID)
	cMsg.Token64 = C.uint64_t(binary.LittleEndian.Uint64(goMsg.Token))

	for _, opt := range goMsg.OptionsRaw() {
		optBytes := opt.ToBytes()
		cOptBytes := C.CBytes(optBytes)
		C.CoAP_AppendOptionToList((**C.CoAP_option_t)(&cMsg.pOptionsList), C.uint16_t(opt.ID), (*C.uint8_t)(cOptBytes), C.uint16_t(len(optBytes)))
		C.free(cOptBytes)
	}
}

// typedef CoAP_HandlerResult_t (* CoAP_ResourceHandler_fPtr_t)(CoAP_Message_t* pReq, CoAP_Message_t* pResp)
//export go_ResourceHandler
func go_ResourceHandler(pReq *C.CoAP_Message_t, pResp *C.CoAP_Message_t) C.CoAP_HandlerResult_t {
	logrus.Info("The handler got called!")

	req := toGoMessage(pReq)
	res := toGoMessage(pResp)

	resource := resources[req.PathString()]
	if resource.Handler != nil {
		result := resource.Handler(req, &res)
		logrus.Info("Prepare response!")

		logrus.WithField("pResp", pResp.Code).Info("pResp")
		toCMessage(res, pResp)

		switch result {
		case OK:
			return C.HANDLER_OK
		case POSTPONE:
			return C.HANDLER_POSTPONE
		case ERROR:
			return C.HANDLER_ERROR
		}
	}
	logrus.WithField("ReqPath", req.PathString()).Error("Missing Handler")
	return C.HANDLER_ERROR
}

func createIpv4Ep(ip net.IP, port int) C.NetEp_t {
	ipAddr := ip.To4()
	addr := C.NetAddr_t{ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3]}

	ep := C.NetEp_t{}
	ep.NetType = C.IPV4
	ep.NetAddr = addr
	ep.NetPort = C.uint16_t(port)
	return ep
}

func createIpv6Ep(ip net.IP, port int) C.NetEp_t {
	ipAddr := ip.To16()
	addr := C.NetAddr_t{
		ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3],
		ipAddr[4], ipAddr[5], ipAddr[6], ipAddr[7],
		ipAddr[8], ipAddr[9], ipAddr[10], ipAddr[11],
		ipAddr[12], ipAddr[13], ipAddr[14], ipAddr[15],
	}

	ep := C.NetEp_t{}
	ep.NetType = C.IPV6
	ep.NetAddr = addr
	ep.NetPort = C.uint16_t(port)
	return ep
}

func createUartEp(comPort byte) C.NetEp_t {
	ep := C.NetEp_t{}
	ep.NetType = C.UART
	ep.NetAddr = C.NetAddr_t{comPort}
	ep.NetPort = C.uint16_t(0)
	return ep
}

func coapEndpoint(cEp C.NetEp_t) CoapEndpoint {
	if cEp.NetType == C.IPV4 {
		pTargetIp := unsafe.Pointer(&cEp.NetAddr)
		targetPort := cEp.NetPort
		ipBytes := C.GoBytes(pTargetIp, 4)
		// TODO: Check IP Byte order!
		return CoapEndpoint{
			Type: EP_IPV4,
			Ip:   net.IP(ipBytes),
			Port: int(targetPort),
		}
	} else if cEp.NetType == C.IPV6 {
		pTargetIp := unsafe.Pointer(&cEp.NetAddr)
		targetPort := cEp.NetPort
		ipBytes := C.GoBytes(pTargetIp, 16)
		// TODO: Check IP Byte order!
		return CoapEndpoint{
			Type: EP_IPV6,
			Ip:   net.IP(ipBytes),
			Port: int(targetPort),
		}
	} else if cEp.NetType == C.UART {
		return CoapEndpoint{
			Type:    EP_UART,
			ComPort: cEp.NetAddr[0],
		}
	} else {
		panic("Not implemented")
		return CoapEndpoint{}
	}
}
