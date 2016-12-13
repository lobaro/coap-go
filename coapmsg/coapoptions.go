// We use this options outside of the package.
// It's more similar to the http Header API
package coapmsg

type OptionValue interface{}

// A CoapOptions represents a option mapping
// keys to sets of values.
type CoapOptions map[OptionID][]OptionValue

// Add adds the key, value pair to the header.
// It appends to any existing values associated with key.
func (h CoapOptions) Add(key OptionID, value OptionValue) {
	h[key] = append(h[key], value)
}

// Set sets the header entries associated with key to
// the single element value. It replaces any existing
// values associated with key.
func (h CoapOptions) Set(key OptionID, value OptionValue) {
	h[key] = []OptionValue{value}
}

// Get gets the first value associated with the given key.
// If there are no values associated with the key, Get returns
// nil. Get is a convenience method. For more
// complex queries, access the map directly.
func (h CoapOptions) Get(key OptionID) OptionValue {
	if h == nil {
		return nil
	}
	v := h[key]
	if len(v) == 0 {
		return nil
	}
	return v[0]
}

// Del deletes the values associated with key.
func (h CoapOptions) Del(key OptionID) {
	delete(h, key)
}
