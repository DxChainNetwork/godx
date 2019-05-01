package storageclient

type ErrSet struct {
	ErrorSet []error
}

func (es ErrSet) Error() string {
	s := "{"
	for i, err := range es.ErrorSet {
		if i != 0 {
			s = s + "; "
		}
		s = s + err.Error()
	}

	return s + "}"
}

func ErrCombine(errs ...error) error {
	var es ErrSet
	for _, err := range errs {
		if err == nil {
			continue
		}
		es.ErrorSet = append(es.ErrorSet, err)
	}
	if len(es.ErrorSet) == 0 {
		return nil
	}
	return es
}
