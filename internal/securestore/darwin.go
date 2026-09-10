//go:build darwin && !nousage

package securestore

import (
	"encoding/base64"
	"encoding/hex"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The path is a private test seam for disposable keychains. Production selects
// the OS default keychain; neither PATH nor environment selects a helper/store.
type darwinStore struct{ keychain string }

func Native() Store { return darwinStore{} }

type darwinAPI struct {
	release         func(uintptr)
	getInteraction  func(*uint8) int32
	setInteraction  func(uint8) int32
	defaultKeychain func(*uintptr) int32
	open            func(string, *uintptr) int32
	status          func(uintptr, *uint32) int32
	find            func(uintptr, uint32, *byte, uint32, *byte, *uint32, *unsafe.Pointer, *uintptr) int32
	add             func(uintptr, uint32, *byte, uint32, *byte, uint32, *byte, *uintptr) int32
	modify          func(uintptr, uintptr, uint32, *byte) int32
	delete          func(uintptr) int32
	freeContent     func(uintptr, unsafe.Pointer) int32
}

var darwinLoadOnce sync.Once
var darwinFunctions *darwinAPI

// Security.framework's legacy interaction preference is process-wide. Serialize
// our native calls and restore the previous preference after each operation.
// Personal legacy go-keyring calls run in a separate OS utility process.
var darwinCallMu sync.Mutex

func loadDarwin() *darwinAPI {
	darwinLoadOnce.Do(func() {
		// A missing symbol/version is unavailable, never a process panic or fallback.
		defer func() {
			if recover() != nil {
				darwinFunctions = nil
			}
		}()
		core, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		security, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			purego.Dlclose(core)
			return
		}
		api := &darwinAPI{}
		purego.RegisterLibFunc(&api.release, core, "CFRelease")
		purego.RegisterLibFunc(&api.getInteraction, security, "SecKeychainGetUserInteractionAllowed")
		purego.RegisterLibFunc(&api.setInteraction, security, "SecKeychainSetUserInteractionAllowed")
		purego.RegisterLibFunc(&api.defaultKeychain, security, "SecKeychainCopyDefault")
		purego.RegisterLibFunc(&api.open, security, "SecKeychainOpen")
		purego.RegisterLibFunc(&api.status, security, "SecKeychainGetStatus")
		purego.RegisterLibFunc(&api.find, security, "SecKeychainFindGenericPassword")
		purego.RegisterLibFunc(&api.add, security, "SecKeychainAddGenericPassword")
		purego.RegisterLibFunc(&api.modify, security, "SecKeychainItemModifyAttributesAndData")
		purego.RegisterLibFunc(&api.delete, security, "SecKeychainItemDelete")
		purego.RegisterLibFunc(&api.freeContent, security, "SecKeychainItemFreeContent")
		darwinFunctions = api
	})
	return darwinFunctions
}

func classifyDarwin(status int32) error {
	switch status {
	case 0:
		return nil
	case -25300:
		return &Error{Missing}
	case -25308, -25315:
		return &Error{Locked}
	case -25293, -128:
		return &Error{Denied}
	default:
		return &Error{Unavailable}
	}
}

func (s darwinStore) operate(fn func(*darwinAPI, uintptr) error) error {
	api := loadDarwin()
	if api == nil {
		return &Error{Unavailable}
	}
	darwinCallMu.Lock()
	defer darwinCallMu.Unlock()
	var interaction uint8
	if err := classifyDarwin(api.getInteraction(&interaction)); err != nil {
		return err
	}
	if err := classifyDarwin(api.setInteraction(0)); err != nil {
		return err
	}
	defer api.setInteraction(interaction)
	var keychain uintptr
	var status int32
	if s.keychain == "" {
		status = api.defaultKeychain(&keychain)
	} else {
		status = api.open(s.keychain, &keychain)
	}
	if err := classifyDarwin(status); err != nil {
		return err
	}
	if keychain == 0 {
		return &Error{Unavailable}
	}
	defer api.release(keychain)
	var state uint32
	if err := classifyDarwin(api.status(keychain, &state)); err != nil {
		return err
	}
	if state&1 == 0 {
		return &Error{Locked}
	} // kSecUnlockStateStatus
	return fn(api, keychain)
}

func decodeDarwinValue(value string) (string, error) {
	if encoded, ok := strings.CutPrefix(value, "go-keyring-base64:"); ok {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", &Error{Unavailable}
		}
		return string(data), nil
	}
	if encoded, ok := strings.CutPrefix(value, "go-keyring-encoded:"); ok {
		data, err := hex.DecodeString(encoded)
		if err != nil {
			return "", &Error{Unavailable}
		}
		return string(data), nil
	}
	return value, nil
}

func (s darwinStore) Get(service, account string) (string, error) {
	if len(service) > 64*1024 || len(account) > 64*1024 {
		return "", &Error{TooLarge}
	}
	svc, acct := []byte(service), []byte(account)
	var value string
	err := s.operate(func(api *darwinAPI, keychain uintptr) error {
		var length uint32
		var data unsafe.Pointer
		status := api.find(keychain, uint32(len(svc)), unsafe.SliceData(svc), uint32(len(acct)), unsafe.SliceData(acct), &length, &data, nil)
		runtime.KeepAlive(svc)
		runtime.KeepAlive(acct)
		if err := classifyDarwin(status); err != nil {
			return err
		}
		defer api.freeContent(0, data)
		if length > 64*1024 {
			return &Error{TooLarge}
		}
		if length != 0 && data == nil {
			return &Error{Unavailable}
		}
		value = string(unsafe.Slice((*byte)(data), int(length)))
		return nil
	})
	if err != nil {
		return "", err
	}
	return decodeDarwinValue(value)
}

func (s darwinStore) Set(service, account, value string) error {
	if len(value) > 64*1024 || len(service) > 64*1024 || len(account) > 64*1024 {
		return &Error{TooLarge}
	}
	svc, acct, data := []byte(service), []byte(account), []byte(value)
	err := s.operate(func(api *darwinAPI, keychain uintptr) error {
		var item uintptr
		status := api.find(keychain, uint32(len(svc)), unsafe.SliceData(svc), uint32(len(acct)), unsafe.SliceData(acct), nil, nil, &item)
		defer func() { runtime.KeepAlive(svc); runtime.KeepAlive(acct); runtime.KeepAlive(data) }()
		if status == -25300 {
			return classifyDarwin(api.add(keychain, uint32(len(svc)), unsafe.SliceData(svc), uint32(len(acct)), unsafe.SliceData(acct), uint32(len(data)), unsafe.SliceData(data), nil))
		}
		if err := classifyDarwin(status); err != nil {
			return err
		}
		if item == 0 {
			return &Error{Unavailable}
		}
		defer api.release(item)
		return classifyDarwin(api.modify(item, 0, uint32(len(data)), unsafe.SliceData(data)))
	})
	if err != nil {
		return err
	}
	got, err := s.Get(service, account)
	if err != nil {
		return err
	}
	if got != value {
		return &Error{Unavailable}
	}
	return nil
}

func (s darwinStore) Delete(service, account string) error {
	if len(service) > 64*1024 || len(account) > 64*1024 {
		return &Error{TooLarge}
	}
	svc, acct := []byte(service), []byte(account)
	return s.operate(func(api *darwinAPI, keychain uintptr) error {
		var item uintptr
		status := api.find(keychain, uint32(len(svc)), unsafe.SliceData(svc), uint32(len(acct)), unsafe.SliceData(acct), nil, nil, &item)
		runtime.KeepAlive(svc)
		runtime.KeepAlive(acct)
		if err := classifyDarwin(status); err != nil {
			return err
		}
		if item == 0 {
			return &Error{Unavailable}
		}
		defer api.release(item)
		return classifyDarwin(api.delete(item))
	})
}
