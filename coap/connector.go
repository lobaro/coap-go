package coap

type SerialConnecter interface {
	Connect(host string) (Connection, error)
}
