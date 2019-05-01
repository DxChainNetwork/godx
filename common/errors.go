package common

type ErrorSet struct {
	ErrSet []error
}

// Error implements the error interface
// convert error into a string
func (es ErrorSet) Error() string {
	s := "{"
	for i, err := range es.ErrSet {
		if i != 0 {
			s = s + "; "
		}
		s = s + err.Error()
	}
	return s + "}"
}

// ErrCompose composes multiple errors into one error set
// follows the input orders
func ErrCompose(errs ...error) error {
	var es ErrorSet
	for _, err := range errs {
		if err == nil {
			continue
		}
		es.ErrSet = append(es.ErrSet, err)
	}
	if len(es.ErrSet) == 0 {
		return nil
	}

	return es
}

// ErrExtend combine multiple errors into one error set
// The input can either be error or errorset, where the extension
// will be located on the end
func ErrExtend(err, extension error) error {
	if err == nil {
		return extension
	}
	if extension == nil {
		return err
	}

	var es ErrorSet

	// check the original error
	switch e := err.(type) {
	case ErrorSet:
		es = e
	default:
		es.ErrSet = []error{e}
	}

	// check the extension
	switch e := extension.(type) {
	case ErrorSet:
		es.ErrSet = append(es.ErrSet, e.ErrSet...)
	default:
		es.ErrSet = append(es.ErrSet, error(e))
	}

	// check the length of the error set
	if len(es.ErrSet) == 0 {
		return nil
	}

	return es
}

// Contains check if the target error can be found in source error
// return true if the error can be found, else, return false
func ErrContains(src, tar error) bool {
	if tar == nil || src == nil {
		return false
	}
	if src == tar {
		return true
	}
	switch e := src.(type) {
	case ErrorSet:
		for _, err := range e.ErrSet {
			if ErrContains(err, tar) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
