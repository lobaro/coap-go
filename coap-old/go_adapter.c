
#include <stdio.h>
#include <time.h>
#include "interface/network/net_Endpoint.h"
#include "interface/network/net_Packet.h"
#include "interface/network/net_Socket.h"
#include "coap.h"
#include "go_adapter.h"

//-------------------------------------------------------------------------
//Implementation for these function prototypes must be provided externally:
//-------------------------------------------------------------------------

//implement internaly used functions (see also lobaro-coap/interface/coap_interface.h)
//Uart/Display function to print debug/status messages to
void hal_debug_puts(char *s) {
	printf("%s", s);
	fflush(stdout);
}
void hal_debug_putc(char c){
	printf("%c", c);
	fflush(stdout);
}

//1Hz Clock used by timeout logic
uint32_t hal_rtc_1Hz_Cnt(void){
 return (unsigned)time(NULL);
}

//Non volatile memory e.g. flash/sd-card/eeprom
//used to store observers during deepsleep of server
uint8_t* hal_nonVolatile_GetBufPtr(){
	return NULL; //not implemented yet
}

bool hal_nonVolatile_WriteBuf(uint8_t* data, uint32_t len){
	return false; //not implemented yet
}

////////////////////
// Setup CoAP stack
////////////////////

//---------------------------------

//CoAP_Result_t  CoAP_Init(uint8_t* pMemory, int16_t MemorySize);


// A message was received, tell the stack to handle it
void CoAP_ReceivedPacket(uint8_t fromIfID, char *pdata, unsigned short len) {
	NetPacket_t		Packet;
	NetSocket_t* 	pSocket=NULL;


//get socket by handle
	pSocket = RetrieveSocket2(fromIfID);

	if(pSocket == NULL){
		ERROR("Corresponding Socket not found!\r\n");
		return;
	}

//packet data
	Packet.pData = pdata;
	Packet.size = len;
	
	// TODO: use real endpoints
	Packet.Sender.NetPort = 5683; 
    Packet.Sender.NetType = IPV4;
	Packet.Receiver.NetPort = pSocket->EpLocal.NetPort;
    Packet.Receiver.NetType = IPV4;

//Sender

//meta info
	Packet.MetaInfo.Type = META_INFO_NONE;

	//call the consumer of this socket
	//the packet is only valid during runtime of consuming function!
	//-> so it has to copy relevant data if needed
	// or parse it to a higher level and store this result!
	// TODO: The packet needs the endpoint, but the endpoint is already at the socket. Handle that internally!
	pSocket->RxCB(pSocket->ifID, &Packet);

	return;
}

void CoAP_ReceivedUdp4Packet(uint8_t fromIfID, NetAddr_t remoteIp, uint16_t remotePort, char *pdata, unsigned short len) {
	NetPacket_t		Packet;
	NetSocket_t* 	pSocket=NULL;


//get socket by handle
	pSocket = RetrieveSocket2(fromIfID);

	if(pSocket == NULL){
		ERROR("Corresponding Socket not found!\r\n");
		return;
	}

//packet data
	Packet.pData = pdata;
	Packet.size = len;
	
    Packet.Sender.NetType = IPV4;
	Packet.Sender.NetAddr = remoteIp;
	Packet.Sender.NetPort = remotePort;
	 
    Packet.Receiver.NetType = IPV4;
	Packet.Receiver.NetAddr = pSocket->EpLocal.NetAddr;
	Packet.Receiver.NetPort = pSocket->EpLocal.NetPort;

//meta info
	Packet.MetaInfo.Type = META_INFO_NONE;

	//call the consumer of this socket
	//the packet is only valid during runtime of consuming function!
	//-> so it has to copy relevant data if needed
	// or parse it to a higher level and store this result!
	// TODO: The packet needs the endpoint, but the endpoint is already at the socket. Handle that internally!
	pSocket->RxCB(pSocket->ifID, &Packet);

	return;
}

// typedef void ( * NetReceiveCallback_fn )(uint8_t ifID, NetPacket_t* pckt);
// Callback can be CoAP_onNewPacketHandler to use the stack
// SendPacket can be CoAP_SendPacket
NetSocket_t* CoAP_CreateInterfaceSocket(uint8_t ifID)
{
	NetSocket_t* pSocket;

	pSocket=RetrieveSocket2(ifID);
	if(pSocket != NULL) {
		ERROR("CoAP_ESP8266_CreateInterfaceSocket(): interface ID already in use!\r\n");
		return NULL;
	}

	pSocket = AllocSocket();
	if(pSocket == NULL){
		ERROR("CoAP_ESP8266_CreateInterfaceSocket(): failed socket allocation\r\n");
		return NULL;
	}

//local side of socket
	pSocket->EpLocal.NetType = IPV4;
	pSocket->EpLocal.NetPort = 5683; // 5683 is the default port due to rfc7252 6.1. (5684 for coaps)

//remote side of socket
	//pSocket->EpRemote.NetType = IPV4;
	//pSocket->EpRemote.NetPort = 5683; 		//varies with requester, not known yet
	//pSocket->EpRemote.NetAddr.IPv4.u32[0] = 1; // TODO real addr

	//pSocket->Handle = (void*) (ifID); //external  to CoAP Stack
	pSocket->ifID = ifID; //internal  to CoAP Stack

//user callback registration
	pSocket->RxCB = CoAP_onNewPacketHandler;
	pSocket->Tx = CoAP_SendPacket;
	pSocket->Alive = true;

	INFO("- CoAP_CreateInterfaceSocket(): listening... IfID: %d \r\n",ifID);
	return pSocket;
}

// Should be handed over to CoAP_CreateInterfaceSocket SendPacket
// Responsible for actually sending the packet
bool CoAP_SendPacket(uint8_t ifID, NetPacket_t* pckt)
{
	NetAddr_t* addr = &pckt->Receiver.NetAddr;
    Go_SendPacket(ifID, (uint8_t*)addr, pckt->Receiver.NetPort, pckt);
}
