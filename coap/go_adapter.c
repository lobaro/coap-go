
#include <stdio.h>
#include <time.h>
#include "interface/network/net_Endpoint.h"
#include "interface/network/net_Packet.h"
#include "interface/network/net_Socket.h"
#include "coap.h"
#include "go_adapter.h"

/*
NetSocket_t* GetFreeInterface() {
}
*/

//-------------------------------------------------------------------------
//Implementation for these function prototypes must be provided externally:
//-------------------------------------------------------------------------

//implement internaly used functions (see also lobaro-coap/interface/coap_interface.h)
//Uart/Display function to print debug/status messages to
void hal_uart_puts(char *s) {
	printf("%s",s);
}
void hal_uart_putc(char c){
	printf("%c",c);
}

//1Hz Clock used by timeout logic
uint32_t hal_rtc_1Hz_Cnt(void){
 return (unsigned)time(NULL);
}

//Non volatile memory e.g. flash/sd-card/eeprom
//used to store observers during deepsleep of server
uint8_t* hal_nonVolatile_GetBufPtr(){
	return NULL; //not implemented yet on esp8266
}

bool hal_nonVolatile_WriteBuf(uint8_t* data, uint32_t len){
	return false; //not implemented yet on esp8266
}

////////////////////
// Setup CoAP stack
////////////////////

//---------------------------------

//CoAP_Result_t  CoAP_Init(uint8_t* pMemory, int16_t MemorySize);

void InitMemory() {
    static uint8_t memmory[4096];
    CoAP_Init(memmory, 4096);
}

// A message was received, tell the stack to handle it
void ReceivedCoapPacket(uint8_t fromIfID, char *pdata, unsigned short len) {
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

//Sender

//meta info
	Packet.MetaInfo.Type = META_INFO_NONE;

	//call the consumer of this socket
	//the packet is only valid during runtime of consuming function!
	//-> so it has to copy relevant data if needed
	// or parse it to a higher level and store this result!
	pSocket->RxCB(pSocket->ifID, &Packet);

	return;
}

NetSocket_t* CreateSocket(uint8_t ifID) {
    return CoAP_CreateInterfaceSocket(ifID, 8081, CoAP_onNewPacketHandler, CoAP_SendPacket);
}

// typedef void ( * NetReceiveCallback_fn )(uint8_t ifID, NetPacket_t* pckt);
// Callback can be CoAP_onNewPacketHandler to use the stack
// SendPacket can be CoAP_SendPacket
NetSocket_t* CoAP_CreateInterfaceSocket(uint8_t ifID, uint16_t LocalPort, NetReceiveCallback_fn Callback, NetTransmit_fn SendPacket)
{
	NetSocket_t* pSocket;

	pSocket=RetrieveSocket2(ifID);
	if(pSocket != NULL) {
		ERROR("CoAP_ESP8266_CreateInterfaceSocket(): interface ID already in use!\r\n");
		return NULL;
	}

	if(Callback == NULL || SendPacket == NULL) {
		ERROR("CoAP_ESP8266_CreateInterfaceSocket(): packet rx & tx functions must be provided!\r\n");
		return NULL;
	}

	pSocket = AllocSocket();
	if(pSocket == NULL){
		ERROR("CoAP_ESP8266_CreateInterfaceSocket(): failed socket allocation\r\n");
		return NULL;
	}

//local side of socket
	//struct ip_info LocalIpInfo;
	pSocket->EpLocal.NetType = IPV4;
	pSocket->EpLocal.NetPort = LocalPort;

//remote side of socket
	pSocket->EpRemote.NetType = IPV4;
	pSocket->EpRemote.NetPort = LocalPort; 		//varies with requester, not known yet
	pSocket->EpRemote.NetAddr.IPv4.u32[0] = 1; // TODO real addr

//assign socket identification IDs (handle=internal, ifID=external)
	//create ESP8266 udp connection

	//pSocket->Handle = (void*) (ifID); //external  to CoAP Stack
	pSocket->ifID = ifID; //internal  to CoAP Stack

//user callback registration
	pSocket->RxCB = Callback;
	pSocket->Tx = SendPacket;
	pSocket->Alive = true;

	INFO("- CoAP_ESP8266_CreateInterfaceSocket(): listening... IfID: %d  Port: %d\r\n",ifID, LocalPort);
	return pSocket;
}

// Should be handed over to CoAP_CreateInterfaceSocket SendPacket
// Responsible for actually sending the packet
bool CoAP_SendPacket(uint8_t ifID, NetPacket_t* pckt)
{
	if(pckt->Receiver.NetType != IPV4){
		ERROR("CoAP_ESP8266_SendDatagram(...): Wrong NetType!\r\n");
		return false;
	}

	NetSocket_t* pSocket;
	pSocket=RetrieveSocket2(ifID);

	if(pSocket == NULL) {
		ERROR("CoAP_ESP8266_SendDatagram(...): InterfaceID not found!\r\n");
		return false;
	}
}
