package storagemanager

import "errors"

var (
	ErrFolderAlreadyExist  = errors.New("storage folder already exist")
	ErrDataFileAlreadExist = errors.New("sector data or metadata already exist in the folder")
	ErrMock                = errors.New("mock failure")
)
