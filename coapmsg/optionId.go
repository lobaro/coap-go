package coapmsg

// OptionID identifies an option in a message.
type OptionId uint16

/*
   +-----+----+---+---+---+----------------+--------+--------+---------+
   | No. | C  | U | N | R | Name           | Format | Length | Default |
   +-----+----+---+---+---+----------------+--------+--------+---------+
   |   1 | x  |   |   | x | If-Match       | opaque | 0-8    | (none)  |
   |   3 | x  | x | - |   | Uri-Host       | string | 1-255  | (see    |
   |     |    |   |   |   |                |        |        | below)  |
   |   4 |    |   |   | x | ETag           | opaque | 1-8    | (none)  |
   |   5 | x  |   |   |   | If-None-Match  | empty  | 0      | (none)  |
   |   7 | x  | x | - |   | Uri-Port       | uint   | 0-2    | (see    |
   |     |    |   |   |   |                |        |        | below)  |
   |   8 |    |   |   | x | Location-Path  | string | 0-255  | (none)  |
   |  11 | x  | x | - | x | Uri-Path       | string | 0-255  | (none)  |
   |  12 |    |   |   |   | Content-Format | uint   | 0-2    | (none)  |
   |  14 |    | x | - |   | Max-Age        | uint   | 0-4    | 60      |
   |  15 | x  | x | - | x | Uri-Query      | string | 0-255  | (none)  |
   |  17 | x  |   |   |   | Accept         | uint   | 0-2    | (none)  |
   |  20 |    |   |   | x | Location-Query | string | 0-255  | (none)  |
   |  35 | x  | x | - |   | Proxy-Uri      | string | 1-1034 | (none)  |
   |  39 | x  | x | - |   | Proxy-Scheme   | string | 1-255  | (none)  |
   |  60 |    |   | x |   | Size1          | uint   | 0-4    | (none)  |
   +-----+----+---+---+---+----------------+--------+--------+---------+
   C=Critical, U=Unsafe, N=NoCacheKey, R=Repeatable
*/

// Option IDs.
//go:generate stringer -type=OptionId
const (
	IfMatch       OptionId = 1
	URIHost       OptionId = 3
	ETag          OptionId = 4
	IfNoneMatch   OptionId = 5
	Observe       OptionId = 6
	URIPort       OptionId = 7
	LocationPath  OptionId = 8
	URIPath       OptionId = 11
	ContentFormat OptionId = 12
	MaxAge        OptionId = 14
	URIQuery      OptionId = 15
	Accept        OptionId = 17
	LocationQuery OptionId = 20
	ProxyURI      OptionId = 35
	ProxyScheme   OptionId = 39
	Size1         OptionId = 60
)

func (o OptionId) Critical() bool {
	return uint16(o)&1 != 0
}

// "Unsafe to forward" proxies will not forward unsafe options
func (o OptionId) UnSafe() bool {
	return uint16(o)&uint16(2) != 0
}

// NoCacheKey only has a meaning for options that are Safe-to-Forward
func (o OptionId) NoCacheKey() bool {
	return bool((o & 0x1e) == 0x1c)
}
