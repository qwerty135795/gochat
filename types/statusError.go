package types

type StatusError struct {
	Err    error
	Status int
}

func (e StatusError) Unwrap() error {
	return e.Err
}
func (e StatusError) HTTPStatus() int {
	return e.Status
}

func (e StatusError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}
