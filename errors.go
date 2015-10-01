package teleport

import "fmt"

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

func IsNotFound(e error) bool {
	_, ok := e.(*NotFoundError)
	return ok
}

type AlreadyExistsError struct {
	Message string
}

func (n *AlreadyExistsError) Error() string {
	if n.Message != "" {
		return n.Message
	} else {
		return "Object already exists"
	}
}

func IsAlredyExists(e error) bool {
	_, ok := e.(*AlreadyExistsError)
	return ok
}

type MissingParameterError struct {
	Param string
}

func (m *MissingParameterError) Error() string {
	return fmt.Sprintf("missing required parameter '%v'", m.Param)
}

type BadParameterError struct {
	Param string
	Err   string
}

func (m *BadParameterError) Error() string {
	return fmt.Sprintf("bad parameter '%v', %v", m.Param, m.Err)
}

type CompareFailedError struct {
	Message string
}

func (e *CompareFailedError) Error() string {
	if e.Message != "" {
		return e.Message
	} else {
		return "Compare failed"
	}

}

func IsCompareFailed(e error) bool {
	_, ok := e.(*CompareFailedError)
	return ok
}

type ReadonlyError struct {
	Message string
}

func (e *ReadonlyError) Error() string {
	if e.Message != "" {
		return e.Message
	} else {
		return "Object works in readonly and can't modify data"
	}

}
