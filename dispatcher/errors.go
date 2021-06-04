package dispatcher

type authFailed struct{}

func (authFailed) Error() string {
	return "Unauthorized"
}

type notImplemented struct{}

func (notImplemented) Error() string {
	return "Not implemented"
}
