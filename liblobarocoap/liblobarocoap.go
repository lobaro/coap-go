package liblobarocoap

/*
#cgo LDFLAGS: -L${SRCDIR} -llobaro_coap
#include "liblobaro_coap.h"
#include <stdio.h>

void go_debug_puts(char *s) {
	printf("%s", s);
	fflush(stdout);
}
void go_debug_putc(char c){
	printf("%c", c);
	fflush(stdout);
}

static inline CoAP_API_t CreateApi() {
	CoAP_API_t api;
	api.debugPuts = go_debug_puts;
	api.debugPutc = go_debug_putc;

	return api;
}


*/
import "C"
import (
	"log"
	"unsafe"
)

var coapMemory = make([]byte, 4096)

func InitMemory() {
	log.Println("Allocated go memory at", unsafe.Pointer(&coapMemory))

	api := C.CreateApi()
	cfg := C.CoAP_Config_t{}

	cfg.Memory = (*C.uint8_t)(unsafe.Pointer(&coapMemory))
	cfg.MemorySize = C.int16_t(len(coapMemory))

	C.CoAP_Init(api, cfg)
}

func doWork() {
	C.CoAP_doWork()
}
