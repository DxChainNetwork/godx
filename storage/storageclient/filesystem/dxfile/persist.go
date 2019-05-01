package dxfile

import (
	"errors"
	"io"
)

// readOverhead read the first two uint64 as header length and segment offset
func readOverhead(r io.Reader) error {

}

func composeError(errs ...error) error {
	var errMsg = "["
	for _, err := range errs {
		if err == nil {
			continue
		}
		if len(errMsg) != 1{
			errMsg += "; "
		}
		errMsg += err.Error()
	}
	errMsg += "]"
	if errMsg == "[]" {
		return nil
	}
	return errors.New(errMsg)
}