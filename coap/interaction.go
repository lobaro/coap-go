package coap

// Interaction tracks one interaction between a CoAP client and Server.
// The Interaction is created with a request and ends with a response.
type Interaction struct {
	req          *Request
	acknowledged bool
}

func NewInteraction(req *Request) *Interaction {
	return &Interaction{
		req: req,
	}
}

func (i *Interaction) Ack() bool {
	return i.acknowledged
}

func (i *Interaction) SetAck() {
	i.acknowledged = true
}
