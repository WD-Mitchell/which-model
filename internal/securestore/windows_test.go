//go:build windows && !nousage

package securestore

import (
	"errors"
	"fmt"
	"github.com/zalando/go-keyring"
	"golang.org/x/sys/windows"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNativeWindowsStore(t *testing.T) {
	nativeTestOnly(t)
	service := fmt.Sprintf("which-model-ci-%d-%d", os.Getpid(), time.Now().UnixNano())
	roundTrip(t, Native(), service)
	if err := Native().Set(service, "oversized", strings.Repeat("x", 2561)); !errors.Is(err, &Error{TooLarge}) {
		t.Fatalf("oversized record: %v", err)
	}
	if _, err := Native().Get(service, "oversized"); !errors.Is(err, &Error{Missing}) {
		t.Fatalf("oversized write created an item: %v", err)
	}
}

func TestWindowsErrorStates(t *testing.T) {
	for _, tc := range []struct {
		err  error
		kind Kind
	}{{keyring.ErrNotFound, Missing}, {windows.ERROR_ACCESS_DENIED, Denied}, {windows.ERROR_LOGON_FAILURE, Denied}, {windows.ERROR_NO_SUCH_LOGON_SESSION, Unavailable}, {keyring.ErrSetDataTooBig, TooLarge}, {errors.New("SYNTHETIC_SECRET_OS_ERROR"), Unavailable}} {
		got := classifyWindows(tc.err)
		if !errors.Is(got, &Error{tc.kind}) || strings.Contains(got.Error(), "SYNTHETIC") {
			t.Fatalf("classification: %v", got)
		}
	}
}
