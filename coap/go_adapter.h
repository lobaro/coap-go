#ifndef GO_ADAPTER_H_
#define GO_ADAPTER_H_

typedef bool ( * Callback_fn )(uint8_t num);
void CallMe(Callback_fn cb);
void CallGo();

// START Exported Go Functions
extern void Go_SendPacket(uint8_t i, NetPacket_t* pckt);
// END Exported Go Functions

NetSocket_t* CoAP_CreateInterfaceSocket(uint8_t ifID);
bool CoAP_SendPacket(uint8_t ifID, NetPacket_t* pckt);
void CoAP_ReceivedPacket(uint8_t fromIfID, char *pdata, unsigned short len);

#endif
