package writeaheadlog

import (
	"io"
	"io/ioutil"
	"os"
)

type (
	utilsSet interface {
		disrupt(string) bool
		readFile(string) ([]byte, error)
		openFile(string, int, os.FileMode) (file, error)
		create(string) (file, error)
		remove(string) error
	}

	file interface {
		io.ReadWriteCloser
		Name() string
		ReadAt([]byte, int64) (int, error)
		Sync() error
		WriteAt([]byte, int64) (int, error)
		Stat() (os.FileInfo, error)
	}
)

// utilsProd is a passthrough to the standard library calls
type utilsProd struct{}

func (*utilsProd) disrupt(string) bool { return false }
func (*utilsProd) readFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}
func (*utilsProd) openFile(path string, flag int, perm os.FileMode) (file, error) {
	return os.OpenFile(path, flag, perm)
}
func (*utilsProd) create(path string) (file, error) {
	return os.Create(path)
}
func (*utilsProd) remove(path string) error {
	return os.Remove(path)
}
