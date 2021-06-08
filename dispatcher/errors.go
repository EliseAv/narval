package dispatcher

type unauthorized struct{}

func (unauthorized) Error() string {
	return "Unauthorized"
}
