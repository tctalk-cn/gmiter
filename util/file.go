package util

import (
	"errors"
	"io"
	"os"
)

func FilePosition(file *os.File) (int64, error) {
	if file == nil {
		return 0, errors.New("null fd when retrieving file position")
	}
	return file.Seek(0, io.SeekCurrent)
}

func FileExists(name string) (b bool, err error) {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
	}
	// Propagates the error if the error is not FileNotExist error.
	return true, err
}

func CreateDirIfNotExists(dirname string) error {
	if _, err := os.Stat(dirname); err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dirname, os.ModePerm)
		}
		return err
	}
	return nil
}

func FileRename(oldName, newName string) bool {
	err := os.Rename(oldName, newName)
	if err != nil {
		return false
	}
	return true
}
