package coap

/*
#cgo CFLAGS: -std=c99

#include "interface/debug/coap_debug.c"
#include "interface/mem/coap_mem.c"
#include "interface/network/net_Endpoint.c"
#include "interface/network/net_Packet.c"
#include "interface/network/net_Socket.c"
#include "option-types/coap_option_blockwise.c"
#include "option-types/coap_option_cf.c"
#include "option-types/coap_option_ETag.c"
#include "option-types/coap_option_observe.c"
#include "option-types/coap_option_uri.c"

#include "go_adapter.h"
*/
import "C"
import (
	"unsafe"
	"log"
	"net"
	"time"
)


/**
typedef struct
{
	SocketHandle_t Handle;  //internal socket handle id chosen by network driver
	uint8_t ifID; 			//external socket/interface id chosen by user

	NetEp_t EpLocal;
	NetEp_t EpRemote;
	NetReceiveCallback_fn RxCB; //callback function on receiving data (normally set to "CoAP_onNewPacketHandler")
	NetTransmit_fn Tx; 			//ext. function called by coap stack to send data after finding socket by ifID (internally)
	bool Alive;
}NetSocket_t;
 */

var running = true

func init() {
	InitMemory()
	go func() {
		for running {
			doWork()
			time.Sleep(time.Millisecond * 100)
		}
		log.Println("Stopped CoAP doWork loop!")
	}()
}

func Stop() {
	running = false
}

var coapMemory = make([]byte, 4096)

type Response struct {
	IfId    uint8
	Message []byte
	Address net.Addr
}

var PendingResponses = make(chan Response, 50)

func InitMemory() {
	log.Println("Allocated go memory at", unsafe.Pointer(&coapMemory))
	C.CoAP_Init((*C.uint8_t)(unsafe.Pointer(&coapMemory)), C.int16_t(len(coapMemory)))
}

func doWork() {
	C.CoAP_doWork()
}

type NetSocket struct {
	Handle uintptr
	IfID   uint8
}

// NetSocket_t* CoAP_CreateInterfaceSocket(uint8_t ifID, uint16_t LocalPort, NetReceiveCallback_fn Callback, NetTransmit_fn SendPacket)
func CreateSocket(ifID uint8) NetSocket {
	socket := C.CoAP_CreateInterfaceSocket(C.uint8_t(ifID))
	return *(*NetSocket)(unsafe.Pointer(socket))
}

// static void udp_recv_cb(char *pdata, unsigned short len) 
// TODO: This is the place where we want to pass the endpoint to the stack, no CreateSocket needed anymore


func HandleReceivedMessage(ifID uint8, msgBytes []byte) {
	C.CoAP_ReceivedPacket(C.uint8_t(ifID), (*C.char)(unsafe.Pointer(&msgBytes[0])), C.ushort(len(msgBytes)))
}

type NetEp_t struct {
	
}

func HandleReceivedUdp4Message(ifID uint8, addr *net.UDPAddr, msgBytes []byte) {
	//o := CoAP_ResOpts_t{}
	//opts := *(*C.CoAP_ResOpts_t)(unsafe.Pointer(&o))
	
	cAdrrBytes := (*C.uint8_t)(C.CBytes(addr.IP.To4()))
	
	// TODO: free CBytes
	C.CoAP_ReceivedUdp4Packet(C.uint8_t(ifID), cAdrrBytes, C.uint16_t(addr.Port), (*C.char)(unsafe.Pointer(&msgBytes[0])), C.ushort(len(msgBytes)))
}

type CoAP_ResOpts_t struct {
	Cf    uint16 //Content-Format
	Flags uint16 //Bitwise resource options //todo: Send Response as CON or NON
	ETag  uint16
};

// CoAP_Res_t* CoAP_CreateResource(char* Uri, char* Descr,CoAP_ResOpts_t Options, CoAP_ResourceHandler_fPtr_t pHandlerFkt, CoAP_ResourceNotifier_fPtr_t pNotifierFkt );
// typedef CoAP_HandlerResult_t (*CoAP_ResourceHandler_fPtr_t)(CoAP_Message_t* pReq, CoAP_Message_t* pResp);
// typedef CoAP_HandlerResult_t (*CoAP_ResourceNotifier_fPtr_t)(CoAP_Observer_t* pListObservers, CoAP_Message_t* pResp);
func CreateResource(uri string, description string) {
	// TODO: C.free the CString - needs stdlib.h

	//o := CoAP_ResOpts_t{}
	//opts := *(*C.CoAP_ResOpts_t)(unsafe.Pointer(&o))
	
	opts := C.CoAP_ResOpts_t{}

	C.CoAP_CreateResource(C.CString(uri), C.CString(description), opts, nil, nil)
	C.CoAP_PrintAllResources()
}

