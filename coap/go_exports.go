package coap

/*

 */
import "C"
import (
	"unsafe"
	"gitlab.com/lobaro/go-c-example/coapmsg"
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

var SendMessageHandler func(ifId uint8, packet coapmsg.Message)

//export Go_SendPacket
func Go_SendPacket(ifId C.uchar, packetPtr unsafe.Pointer) {
	p := *(*netPacket)(packetPtr)
	msgBytes := make([]byte, p.size)
	for i := uint16(0); i < p.size; i++ {
		msgBytes[i] =  *(*byte)(unsafe.Pointer(uintptr(p.data) + uintptr(i)))
	}

	msg, err := coapmsg.ParseMessage(msgBytes)
	if err != nil {
		panic(err)
	}

	if SendMessageHandler == nil {
		panic("You have to set coap.SendMessageHandler")
	}
	// TODO: Use channel
	SendMessageHandler(uint8(ifId), msg)
}
