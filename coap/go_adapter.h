#ifndef GO_ADAPTER_H_
#define GO_ADAPTER_H_

NetSocket_t* CreateSocket(uint8_t ifID);
void InitMemory();

NetSocket_t* CoAP_CreateInterfaceSocket(uint8_t ifID, uint16_t LocalPort, NetReceiveCallback_fn Callback, NetTransmit_fn SendPacket);
bool CoAP_SendPacket(uint8_t ifID, NetPacket_t* pckt);
void ReceivedCoapPacket(uint8_t fromIfID, char *pdata, unsigned short len);

#endif
