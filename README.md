# Lobaro CoAP GoLang adapter

[Lobaro CoAP](https://gitlab.com/lobaro/lobaro-coap) provides a highly portable CoAP stack for Client and Server running on almost any hardware.

The **GoLang adapter** uses cgo to provide a CoAP stack based on the code of Lobaro CoAP. It can be used for **testing the stack on a PC** and to **write server applications in go** that can handle CoAP connections.

## Getting Started

```
go get -u gitlab.com/lobaro/lobaro-coap-go
```

Execute tests
```
go test gitlab.com/lobaro/lobaro-coap-go
```

To use the library in your project, just import
```
import "gitlab.com/lobaro/lobaro-coap-go/coap"
```
