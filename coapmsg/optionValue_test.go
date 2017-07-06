package coapmsg

import (
	"fmt"
	"testing"
)

var numbers = []struct {
	Num        OptionId
	Critical   bool
	Unsafe     bool
	NoCahceKey bool
}{
	{1, true, false, false},
	{3, true, true, false},
	{4, false, false, false},
	{5, true, false, false},
	{7, true, true, true},
	{8, false, false, false},
	{11, true, true, true},

	{12, false, false, false},
	{14, false, true, true},
	{15, true, true, true},
	{17, true, false, false},
	{20, false, false, false},
	{35, true, true, true},
	{39, true, true, true},
	{60, false, false, true},

	// Custom options by Lobaro
	{3000, false, false, false},
	{3008, false, false, false},
	{3012, false, false, false},
	{3016, false, false, false},
	{3020, false, false, false},
}

func TestNumbers(t *testing.T) {
	for _, n := range numbers {
		id := n.Num

		if n.Critical != id.Critical() {
			t.Error(fmt.Sprint("Option ", n.Num, " Critical does not match, should be ", n.Critical))
		}
		if n.Unsafe != id.UnSafe() {
			t.Error(fmt.Sprint("Option ", n.Num, " UnSafe does not match, should be ", n.Unsafe))
		}
		// NoCacheKey only has a meaning for options that are Safe-to-Forward
		if !id.UnSafe() && n.NoCahceKey != id.NoCacheKey() {
			t.Error(fmt.Sprint("Option ", n.Num, " NoCacheKey does not match, should be ", n.NoCahceKey))
		}
	}
}

func TestParsing(t *testing.T) {
	msg := NewMessage()

	if msg.Options().Get(Observe).IsSet() {
		t.Error("Expected not existing option to be not set")
	}

	// Setting to nil is totally valid and sets the option
	err := msg.Options().Set(Observe, nil)

	if err != nil {
		t.Error(err)
	}
	if !msg.Options().Get(Observe).IsSet() {
		t.Error("Expected observe option to be set")
	}

	msg.Options().Set(Observe, 5)

	if !msg.Options().Get(Observe).IsSet() {
		t.Error("Expected observe option to be set")
	}
	if msg.Options().Get(Observe).AsUInt8() != 5 {
		t.Error("Expected observe option to be 5")
	}
	if msg.Options().Get(Observe).AsBytes()[0] != 5 {
		t.Error("Expected observe option to be 5")
	}
	if msg.Options().Get(Observe).AsUInt16() != 5 {
		t.Error("Expected observe option to be 5")
	}
	if msg.Options().Get(Observe).AsUInt32() != 5 {
		t.Error("Expected observe option to be 5")
	}
	if msg.Options().Get(Observe).AsUInt64() != 5 {
		t.Error("Expected observe option to be 5")
	}

	msg.Options().Add(Observe, 6)

	if !msg.Options().Get(Observe).IsSet() {
		t.Error("Expected observe option to be set")
	}
	if msg.Options().Get(Observe).AsUInt8() != 5 {
		t.Errorf("Expected observe option to be 5 but is %v", msg.Options().Get(Observe))
	}

	// The add has a little nested effect
	if msg.Options()[Observe].values[1].AsBytes()[0] != 6 {
		t.Errorf("Expected observe option to be 6 but is %v", msg.Options().Get(Observe))
	}

	msg.Options().Del(Observe)
	if msg.Options().Get(Observe).IsSet() {
		t.Error("Expected deleted existing option to be not set")
	}
}

func TestPrettyPrint_NoChecks(t *testing.T) {
	msg := NewMessage()
	msg.SetPathString("/foo/bar")
	msg.Options().Set(ETag, []byte{1, 2, 3}) // Opaque option
	msg.Options().Add(ETag, []byte{4, 5, 6}) // Opaque option
	msg.Options().Set(IfNoneMatch, 1)        // Empty option
	msg.Options().Set(Observe, 10)           // Uint
	msg.Options().Add(Observe, 11)           // Uint

	t.Log("Path:", msg.Options().Get(URIPath).String())
	t.Log("ETag:", msg.Options().Get(ETag).String())
	t.Log("IfNoneMatch:", msg.Options().Get(IfNoneMatch).String())
	t.Log("Observe:", msg.Options().Get(Observe).String())
}

func _TestFindNumbers(t *testing.T) {
	for i := 3000; i < 3200; i++ {
		id := OptionId(i)
		if id.Critical() {
			continue
		}
		if id.UnSafe() {
			continue
		}

		if id.NoCacheKey() {
			continue
		}

		t.Log(fmt.Sprint(id, ": ", id.Critical(), "\t", id.UnSafe(), "\t", id.NoCacheKey()))
	}
}
