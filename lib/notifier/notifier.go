package notifier

type Notifier interface {
	Notify(title, message string)
}

// NoopNotifier - для тестирования или когда уведомления не нужны
type NoopNotifier struct{}

func NewNoop() Notifier { return &NoopNotifier{} }

func (n *NoopNotifier) Notify(title, message string) {
	// do nothing
}
