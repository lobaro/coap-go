# Lobaro CoAP for GoLang

This repository contains two parts: The **CoAP Client** and a **GoLang Server adapter**. The client focuses on RS232 connections where data is send via [Slip](https://tools.ietf.org/rfc/rfc1055.txt) and [SlipMUX](https://tools.ietf.org/html/draft-bormann-t2trg-slipmux-01)

# GoLang CoAP RS232 Client 
Currently only RS232 is supported. Transports for other Protocols like TCP/IP and UDP are planned.

**Install:**
```
go get -u github.com/lobaro/coap-go/coap
```

The package is strctured similiar to the http package and tries to follow go idematic coding styles.

# GoLang Server adapter for lobaro-coap

**A word of warning:** While the C code works fine and is already used in several projects, the Go wrapper is still under construction. We might even considder to reimplement the server in Go rather than wrapping the C code.

[Lobaro CoAP](https://github.com/lobaro/lobaro-coap) provides a highly portable CoAP stack for Client and Server running on almost any hardware.

The **GoLang adapter** uses cgo to provide a CoAP stack based on the code of Lobaro CoAP. It can be used for **testing the stack on a PC** and to **write server applications in go** that can handle CoAP connections.

## Getting Started

The project consists of multiple submodules:

* **coap** - A pure Go client library with an API similar to Go's http package. Supports multiple Transports (e.g. RS232).
* **liblobarocoap** - A CGO wrapper around [Lobaro CoAP](https://github.com/lobaro/lobaro-coap) C Implementation.
* **coapmsg** The underlying CoAP message structure used by other packages. Based on [dustin/go-coap](https://github.com/dustin/go-coap).

It is planned to extend the `coap` package to support more transports like UDP, TCP in future. The package will also get some code to setup CoAP servers. First based on `liblobarocoap` and later also in native Go.

Contributions are welcome!

### Prerequisite 
To build the project you need a C compiler and the matching [Go](https://golang.org/dl/) toolkit installed. 

For Windows you can use [MinGW](http://www.mingw.org/) to install the gcc. 

When you have a 32 bit C compiler make sure you also use 32 bit Go. Else cgo will not be able to compile the C code.

### Install the code

```
go get -u github.com/lobaro/coap-go
```

Execute tests
```
go test github.com/lobaro/coap-go
```

To use the library in your project, just import
```
import "github.com/lobaro/coap-go"
```

# Usage

## Observe

```
// Start observing, res is the result of a GET request
res, err := coap.Observe(url)
if err != nil {
	panic(err)
}
// Gracefully cancel the observe at the end of this function
defer coap.CancelObserve(res)
var timeoutCh time.After(60 * time.Second)

for {
	select {
	// res.Next returns a response with a new Next channel
	// as soon as the client receives a notification from the server
	case nextRes, ok := <-res.Next:
		if !ok {
			return
		}
		res = nextRes // update res for next interation
		// Handle the Notification (res)
	case <-time.After(20 * time.Second):
		return // Cancel observe after 20 seconds of silence
	case <-timeoutCh:
		return // Cancel observe after 60 seconds
	}
}
```
