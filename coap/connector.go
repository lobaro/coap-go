package coap

type SerialConnecter interface {
	Connect(host string) (*serialConnection, error)
}
