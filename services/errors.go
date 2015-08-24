package services

import "github.com/gravitational/teleport/backend"

type AlreadyAcquiredError struct {
	Message string
}

func (e *AlreadyAcquiredError) Error() string {
	if e.Message != "" {
		return e.Message
	} else {
		return "Lock is already aquired"
	}

}

type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	if e.Message != "" {
		return e.Message
	} else {
		return "Object not found"
	}

}

func convertErr(e error) error {
	if e == nil {
		return nil
	}
	switch e.(type) {
	case *backend.NotFoundError:
		return &NotFoundError{}
	}
	return e
}
