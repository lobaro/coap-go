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
	"log"
	"unsafe"
	"reflect"
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



func init() {
	InitMemory()
}

func InitMemory() {
	C.InitMemory()
}

type NetSocket struct {
	Handle uintptr
	IfID uint8
}

// NetSocket_t* CoAP_CreateInterfaceSocket(uint8_t ifID, uint16_t LocalPort, NetReceiveCallback_fn Callback, NetTransmit_fn SendPacket)
func CreateSocket(ifID uint8) NetSocket {
	socket := C.CreateSocket(C.uint8_t(ifID))
	t := reflect.TypeOf(socket)
	log.Println("socket type:", t.String())
	log.Println("Socket:", socket.ifID)
	
	return *(*NetSocket)(unsafe.Pointer(socket))
}

// static void udp_recv_cb(char *pdata, unsigned short len) 
func FakeReceivedAckPacketFrom(ifID uint8) {
	msg := NewMessage()
	msg.Type = CON

	msgBytes, err := msg.Bytes()
	if err != nil {
		panic(err)
	}
	C.ReceivedCoapPacket(C.uint8_t(ifID), (*C.char)(unsafe.Pointer(&msgBytes[0])), C.ushort(len(msgBytes)))
}
