package coap

type coapError struct {
	err     string
	timeout bool
}

func (e *coapError) Error() string {
	return e.err
}
func (e *coapError) Timeout() bool {
	return e.timeout
}
func (e *coapError) Temporary() bool {
	return true
}
