package clipboard

type Clipboard interface {
	Get() (string, error)
	Set(string) error
}

func New() Clipboard {
	return newPlatformClipboard()
}
