// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"golang.org/x/crypto/sha3"
	"golang.org/x/crypto/twofish"
)

type backupHeader struct {
	Version    string `json:"version"`
	Encryption string `json:"encryption"`
	IV         []byte `json:"iv"`
}

// CreateBackup will create backup of dxfiles
func (client *StorageClient) CreateBackup(dst string, secret []byte) error {
	if err := client.tm.Add(); err != nil {
		return err
	}
	defer client.tm.Done()

	// create gzip file
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := io.Writer(file)

	// initialize backup header
	bh := backupHeader{
		Version:    encryptionVersion,
		Encryption: encryptionPlaintext,
	}

	// if secret is provided, generate new cipher
	bh.IV = make([]byte, twofish.BlockSize)
	if secret != nil {
		bh.Encryption = encryptionTwofish
		_, err := rand.Read(bh.IV)
		if err != nil {
			return err
		}
		c, err := twofish.NewCipher(secret)
		sw := cipher.StreamWriter{
			S: cipher.NewCTR(c, bh.IV),
			W: archive,
		}
		archive = sw
	}

	// leave space for the hash
	if _, err := file.Seek(common.HashLength, io.SeekStart); err != nil {
		return err
	}

	// encode and write header into file
	enc := json.NewEncoder(file)
	if err := enc.Encode(bh); err != nil {
		return err
	}

	// data encryption and write into file
	h := sha3.NewLegacyKeccak256()
	archive = io.MultiWriter(archive, h)
	gzw := gzip.NewWriter(archive)
	tw := tar.NewWriter(gzw)
	if err := client.managedTarDxFiles(tw); err != nil {
		// TODO: ErrCompose Method
		//twErr := tw.Close()
		//gzwErr := gzw.Close()
		//return ErrCombine(err, twErr, gzwErr)
		return nil
	}

	//twErr := tw.Close()
	//gzwErr := gzw.Close()

	_, err = file.WriteAt(h.Sum(nil), 0)
	// TODO: ErrCompose Method
	//return ErrCombine(err, twErr, gzwErr)
	return nil
}

// LoadBackup will load the backup created previously, and restore them back to the original
// files and directory
func (client *StorageClient) LoadBackup(src string, secret []byte) error {
	if err := client.tm.Add(); err != nil {
		return err
	}
	defer client.tm.Done()

	// open the file
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := io.Reader(file)

	// read the hash
	var hash common.Hash
	_, err = io.ReadFull(file, hash[:])
	if err != nil {
		return err
	}

	// read header
	dec := json.NewDecoder(archive)
	var bh backupHeader
	if err := dec.Decode(&bh); err != nil {
		return err
	}

	// body offset
	var off int64
	if buf, ok := dec.Buffered().(*bytes.Reader); ok {
		off, err = file.Seek(int64(1-buf.Len()), io.SeekCurrent)
		if err != nil {
			return err
		}
	} else {
		log.Crit("Buffered should return a bytes.Reader")
	}

	// check the version number
	if bh.Version != encryptionVersion {
		return errors.New("wrong version")
	}

	// wrap with the streamcipher
	archive, err = wrapReaderInCipher(file, bh, secret)
	if err != nil {
		return err
	}

	// get the remaining file, verify if the hash is correct
	h := sha3.NewLegacyKeccak256()
	_, err = io.Copy(h, archive)
	if err != nil {
		return err
	}

	// hash verification
	if !bytes.Equal(h.Sum(nil), hash[:]) {
		return errors.New("file hash does not match")
	}

	// get back to the beginning of the file
	if _, err := file.Seek(off, io.SeekStart); err != nil {
		return err
	}

	// wrap the file
	archive, err = wrapReaderInCipher(file, bh, secret)
	if err != nil {
		return err
	}

	gzr, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	return untarDir(tr, client.staticFilesDir)

}

func (client *StorageClient) managedTarDxFiles(tw *tar.Writer) error {
	return filepath.Walk(client.staticFilesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) != storage.DxFileExt {
			return nil
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		realpath := strings.TrimPrefix(path, client.staticFilesDir)
		header.Name = realpath
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// TODO (mzhang): get DxFile, wait for file management feature to be finished by Jacky

		// TODO (mzhang): open the file from the file set based on the dxpath, wait for file management feature to be
		// finished by Jacky

		return nil
	})
}

func wrapReaderInCipher(r io.Reader, bh backupHeader, secret []byte) (io.Reader, error) {
	switch bh.Encryption {
	case encryptionTwofish:
		c, err := twofish.NewCipher(secret)
		if err != nil {
			return nil, err
		}
		return cipher.StreamReader{
			S: cipher.NewCTR(c, bh.IV),
			R: r,
		}, nil
	case encryptionPlaintext:
		return r, nil
	default:
		return nil, errors.New("unknown cipher")
	}
}

func untarDir(tr *tar.Reader, dstFolder string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		dst := filepath.Join(dstFolder, header.Name)

		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(dst, info.Mode()); err != nil {
				return err
			}
			continue
		}

		dst = uniqueFilename(dst)
		file, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(file, tr)

		_ = file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func uniqueFilename(dst string) string {
	suffix := ""
	counter := 1
	extension := filepath.Ext(dst)
	nameNoExt := strings.TrimSuffix(dst, extension)

	for {
		path := nameNoExt + suffix + extension
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		suffix = fmt.Sprintf("_%v", counter)
		counter++
	}
}
