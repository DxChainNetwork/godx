package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/sha3"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

const (
	tempSuffix         = "_temp" // temporary file suffix
	jsonHashValSize    = 67      // quote + 64 byte hash + quote + \n
	jsonManualHashSize = 9       // quote + len("manual") + quote + \n
)

var (
	ErrBadFilenameSuffix = errors.New("filename suffix '_temp' is not allowed")
	ErrFileInUse         = errors.New("another routine is saving or loading this file")
	ErrBadHeader         = errors.New("wrong header")
	ErrBadVersion        = errors.New("incompatible file version")
	ErrFileOpen          = errors.New("failed to open the file")
)

var (
	activeFiles   = make(map[string]struct{})
	activeFilesMu sync.Mutex
)

type Metadata struct {
	Header, Version string
}

func SaveJSONCompat(meta Metadata, filename string, val interface{}) error {
	// validate file name and whether the file is occupied
	err := fileValidation(filename)
	if err != nil {
		return err
	}

	// remove the file from the list
	defer func() {
		activeFilesMu.Lock()
		delete(activeFiles, filename)
		activeFilesMu.Unlock()
	}()

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)

	// encode metadata into buffer
	if err := enc.Encode(meta.Header); err != nil {
		return errors.New("failed to encode the metadata header: " + err.Error())
	}
	if err := enc.Encode(meta.Version); err != nil {
		return errors.New("failed to encode the metadata version: " + err.Error())
	}

	// marshal the value
	valBytes, err := json.MarshalIndent(val, "", "\t")
	if err != nil {
		return errors.New("failed to marshal the data: " + err.Error())
	}

	// create hashVal, save it into buffer
	hashVal := dataHash(valBytes)
	if err := enc.Encode(hashVal); err != nil {
		return errors.New("failed to encode the checksum: " + err.Error())
	}

	// save the value into the buffer, transfer to byte slice
	buf.Write(valBytes)
	data := buf.Bytes()

	// write data to temp file if data integrity check passed
	if !verifyHash(filename) {
		err = writeFile(filename+tempSuffix, data)
		if err != nil {
			return errors.New("temp file -- " + err.Error())
		}
	}

	err = writeFile(filename, data)
	if err != nil {
		return errors.New("persist file -- " + err.Error())
	}

	return err
}

func fileValidation(filename string) error {
	// verify the filename do not have _temp as suffix
	if strings.HasSuffix(filename, tempSuffix) {
		return ErrBadFilenameSuffix
	}

	activeFilesMu.Lock()
	defer activeFilesMu.Unlock()

	if _, exists := activeFiles[filename]; exists {
		return ErrFileInUse
	}
	activeFiles[filename] = struct{}{}
	return nil
}

func writeFile(filename string, data []byte) (ferr error) {
	// open / create file
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		err = errors.New("failed to open the file: " + err.Error())
		return err
	}

	// error encountered while closing the file
	defer func() {
		closeError := file.Close()
		if closeError != nil && err != nil {
			ferr = errors.New(err.Error() + closeError.Error())
		} else if closeError != nil && err == nil {
			ferr = closeError
		}
	}()

	// Write data into file and save it on the disk
	_, err = file.Write(data)
	if err != nil {
		err = errors.New("failed to write the file: " + err.Error())
		return err
	}
	err = file.Sync()
	if err != nil {
		err = errors.New("failed to sync the file: " + err.Error())
		return err
	}

	return nil
}

func verifyHash(filename string) bool {
	// open the file
	file, err := os.Open(filename)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		return false
	}
	defer file.Close()

	// acquire header and version of the file
	var header, version string
	dec := json.NewDecoder(file)
	if err := dec.Decode(&header); err != nil {
		return false
	}
	if err := dec.Decode(&version); err != nil {
		return false
	}

	// read the rest of the file from the buffer
	remaining, err := ioutil.ReadAll(dec.Buffered())
	if err != nil {
		return false
	}

	// making sure all data from the file are acquired
	extra, err := ioutil.ReadAll(file)
	if err != nil {
		return false
	}
	remaining = append(remaining, extra...)

	// verify the hashVal
	var hashVal Hash
	if len(remaining) >= jsonHashValSize {
		err = json.Unmarshal(remaining[:jsonHashValSize], &hashVal)
		if err == nil {
			return hashVal == dataHash(remaining[68:])
		}
	}

	// hashVal verification failed, check the "manual"
	var manualHash string
	if len(remaining) >= jsonManualHashSize {
		err = json.Unmarshal(remaining[:jsonManualHashSize], &manualHash)
		if err == nil {
			return manualHash == "manual"
		}
	}

	return json.Valid(remaining)
}

func dataHash(data ...[]byte) (h Hash) {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	d.Sum(h[:0])
	return h
}
