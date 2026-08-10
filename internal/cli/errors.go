package cli

// UserError is the user-facing error type — its message is printed to stderr and
// causes exit code 1. An empty Msg means the command already printed its own
// detail and only the exit code matters.
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }
