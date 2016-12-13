package coap

/*

 */
import "C"
import (
	"net"
	"unsafe"
)

/*
typedef struct
{
	uint8_t* pData;
	uint16_t size;
	NetEp_t Sender;
	NetEp_t Receiver;
	MetaInfo_t MetaInfo;
} NetPacket_t;
*/

type netPacket struct {
	data unsafe.Pointer
	size uint16
}

// uint8_t i, NetPacket_t* pckt
//export Go_SendPacket
func Go_SendPacket(ifId C.uchar, targetIp unsafe.Pointer, port uint16, packetPtr unsafe.Pointer) {
	ipBytes := C.GoBytes(targetIp, 4)

	addr := net.UDPAddr{
		IP:   net.IP(ipBytes),
		Port: int(port),
	}

	// TODO use C.NetPacket_t or similiar for safer cast
	p := *(*netPacket)(packetPtr)
	msgBytes := C.GoBytes(p.data, C.int(p.size))
	PendingResponses <- Response{
		IfId:    uint8(ifId),
		Message: msgBytes,
		Address: &addr,
	}
}
