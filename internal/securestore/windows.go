//go:build windows && !nousage

package securestore

import (
	"errors"
	"syscall"

	"github.com/danieljoos/wincred"
	"github.com/zalando/go-keyring"
	"golang.org/x/sys/windows"
)

type windowsStore struct{}

func Native() Store { return windowsStore{} }

func classifyWindows(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, keyring.ErrNotFound), errors.Is(err, syscall.ERROR_NOT_FOUND):
		return &Error{Missing}
	case errors.Is(err, windows.ERROR_ACCESS_DENIED), errors.Is(err, windows.ERROR_LOGON_FAILURE):
		return &Error{Denied}
	case errors.Is(err, keyring.ErrSetDataTooBig):
		return &Error{TooLarge}
	default:
		return &Error{Unavailable}
	}
}

func (windowsStore) Get(service, account string) (string, error) {
	value, err := keyring.Get(service, account)
	if err != nil {
		return "", classifyWindows(err)
	}
	return value, nil
}
func (windowsStore) Set(service, account, value string) error {
	return classifyWindows(keyring.Set(service, account, value))
}
func (windowsStore) Delete(service, account string) error {
	// Delete the named owned entry directly, without retrieving its secret blob.
	return classifyWindows(wincred.NewGenericCredential(service + ":" + account).Delete())
}
