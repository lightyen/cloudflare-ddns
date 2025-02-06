package log

type TracedError interface {
	Error() string
	Stack() string
}

func AsTracedError(err error) (TracedError, bool) {
	u, ok := err.(TracedError)
	if !ok {
		return nil, false
	}
	return u, true
}
