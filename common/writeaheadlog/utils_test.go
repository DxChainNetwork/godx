package writeaheadlog

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
)

func scrambleData(d []byte) []byte {
	randomData := make([]byte, len(d))
	_, err := rand.Read(randomData)
	if err != nil {
		panic("cannot create random data")
	}
	scrambled := make([]byte, len(d), len(d))
	for i := 0; i < len(d); i++ {
		if rand.Intn(4) == 0 {
			scrambled[i] = randomData[i]
		} else {
			scrambled[i] = d[i]
		}
	}
	return scrambled
}

type utilsFaultyDisk struct {
	disabled bool
	failed bool
	failDenominator int
	totolWrites int
	writeLimit int

	mu sync.Mutex
}

type faultyFile struct {
	u    *utilsFaultyDisk
	file *os.File
}

func (f *faultyFile) Read(p []byte) (int, error) {
	return f.file.Read(p)
}
func (f *faultyFile) Write(p []byte) (int, error) {
	f.u.mu.Lock()
	defer f.u.mu.Unlock()
	if f.u.tryFail() {
		return f.file.Write(scrambleData(p))
	}
	return f.file.Write(p)
}
func (f *faultyFile) Close() error { return f.file.Close() }
func (f *faultyFile) Name() string {
	return f.file.Name()
}
func (f *faultyFile) ReadAt(p []byte, off int64) (int, error) {
	return f.file.ReadAt(p, off)
}
func (f *faultyFile) WriteAt(p []byte, off int64) (int, error) {
	f.u.mu.Lock()
	defer f.u.mu.Unlock()
	if f.u.tryFail() {
		return f.file.WriteAt(scrambleData(p), off)
	}
	return f.file.WriteAt(p, off)
}
func (f *faultyFile) Stat() (os.FileInfo, error) {
	return f.file.Stat()
}

func (f *faultyFile) Sync() error {
	f.u.mu.Lock()
	defer f.u.mu.Unlock()
	if f.u.tryFail() {
		return errors.New("could not write to disk (faultyDisk)")
	}
	return f.file.Sync()
}

func newUtilsFaultyDisk(writeLimit int) *utilsFaultyDisk {
	return &utilsFaultyDisk{
		writeLimit: writeLimit,
		failDenominator: 1,
	}
}

// newFaultyFile creates a new faulty file around the provided file handle.
func (u *utilsFaultyDisk) newFaultyFile(f *os.File) *faultyFile {
	return &faultyFile{u: u, file: f}
}

// tryFail will determine whether the operation could be processed
func (u *utilsFaultyDisk) tryFail() bool {
	u.totolWrites++
	if u.disabled {
		return false
	}
	// No consecutive failed
	if u.failed {
		return true
	}

	u.failDenominator += 5
	fail := rand.Intn(int(u.failDenominator)) == 0
	if fail || u.totolWrites > u.writeLimit {
		u.failed = true
		return true
	}
	return false
}

func (u *utilsFaultyDisk) create(path string) (file, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.tryFail() {
		return nil, errors.New("failed to create file (faulty disk)")
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return u.newFaultyFile(f), nil
}

func (u *utilsFaultyDisk) disable() {
	u.mu.Lock()
	u.disabled = true
	u.mu.Unlock()
}

func (*utilsFaultyDisk) disrupt(s string) bool {
	return s == "FaultyDisk"
}

func (u *utilsFaultyDisk) enable() {
	u.mu.Lock()
	u.disabled = false
	u.mu.Unlock()
}

func (u *utilsFaultyDisk) readFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

func (u *utilsFaultyDisk) remove(path string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.tryFail(){
		return nil
	}
	return os.Remove(path)
}

func (u *utilsFaultyDisk) reset() {
	u.mu.Lock()
	u.failDenominator = 0
	u.failed = false
	u.mu.Unlock()
}

func (u *utilsFaultyDisk) openFile(path string, flag int, perm os.FileMode) (file, error) {
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	return u.newFaultyFile(f), nil
}

type utilsCommitFail struct {
	utilsProd
}

func (*utilsCommitFail) disrupt(s string) bool {
	if s == "CommitFail" {
		return true
	}
	return false
}

type utilsRecoveryFail struct {
	utilsProd
}

func (utilsRecoveryFail) disrupt(s string) bool {
	if s == "RecoveryFail" {
		return true
	}
	if s == "UncleanShutdown" {
		return true
	}
	return false
}

type utilsReleaseFail struct {
	utilsProd
}

func (*utilsReleaseFail) disrupt(s string) bool {
	if s == "ReleaseFail" {
		return true
	}
	return false
}

type utilsUncleanShutdown struct {
	utilsProd
}

func (utilsUncleanShutdown) disrupt(s string) bool {
	if s == "UncleanShutdown" {
		return true
	}
	return false
}
