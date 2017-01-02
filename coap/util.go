package coap

import (
	"net/url"
	"runtime"
	"strings"
)

func nop() {}

// Return value if nonempty, def otherwise.
func valueOrDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}

// Given a string of the form "host", "host:port", or "[ipv6::address]:port",
// return true if the string includes a port.
func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }

var portMap = map[string]string{
	"coap":  "5683",
	"coaps": "5684",
}

// canonicalAddr returns url.Host but always with a ":port" suffix
func canonicalAddr(url *url.URL) string {
	addr := url.Host
	if !hasPort(addr) {
		return addr + ":" + portMap[url.Scheme]
	}
	return addr
}

// removeEmptyPort strips the empty port in ":port" to ""
// as mandated by RFC 3986 Section 6.2.3.
// We just do the same for CoAP as it is done for HTTP
func removeEmptyPort(host string) string {
	if hasPort(host) {
		return strings.TrimSuffix(host, ":")
	}
	return host
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
