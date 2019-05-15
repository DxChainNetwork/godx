package storagemanager

import "errors"

var (
	// ErrFolderAlreadyExist represent error of folder already exist
	ErrFolderAlreadyExist = errors.New("storage folder already exist")
	// ErrDataFileAlreadyExist represent error of sector data and metadata already exist
	ErrDataFileAlreadyExist = errors.New("sector data or metadata already exist in the folder")
	// ErrMock disrupt and return error, to test the behavior of system when encounter an error
	ErrMock = errors.New("mock failure")
)
