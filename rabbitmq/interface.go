package rabbitmq

type Publisher interface {
	Publish(message []byte) error
}