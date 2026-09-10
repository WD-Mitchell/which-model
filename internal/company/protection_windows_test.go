//go:build windows

package company

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsDescriptorAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, sddl         string
		directory, allowed bool
	}{
		{"protected file", "O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FR;;;BU)", false, true},
		{"untrusted owner", "O:BUG:SYD:P(A;;FA;;;SY)(A;;FR;;;BU)", false, false},
		{"null DACL", "O:SYG:SY", false, false},
		{"world write", "O:SYG:SYD:P(A;;FA;;;SY)(A;;GW;;;WD)", false, false},
		{"world ACL edit", "O:SYG:SYD:P(A;;FA;;;SY)(A;;WD;;;WD)", false, false},
		{"world delete child", "O:SYG:SYD:P(A;;FA;;;SY)(A;;0x0040;;;WD)", true, false},
		{"ProgramData child creation and metadata", "O:SYG:SYD:P(A;;FA;;;SY)(A;;FR;;;BU)(A;CI;0x0116;;;BU)", true, true},
		{"file metadata write", "O:SYG:SYD:P(A;;FA;;;SY)(A;;0x0110;;;BU)", false, false},
		{"ancestor sibling creation", "O:SYG:SYD:P(A;;FA;;;SY)(A;;0x0004;;;BU)", true, true},
		{"file append", "O:SYG:SYD:P(A;;FA;;;SY)(A;;0x0004;;;BU)", false, false},
		{"deny does not mask unsafe allowance", "O:SYG:SYD:P(D;;GW;;;WD)(A;;GW;;;WD)", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(tc.sddl)
			if err != nil {
				t.Fatal(err)
			}
			if got := trustDescriptor(sd, tc.directory) == nil; got != tc.allowed {
				t.Fatalf("allowed=%v want %v", got, tc.allowed)
			}
		})
	}
}

func TestTrustedInstallerIdentityFromOS(t *testing.T) {
	sid, _, _, err := windows.LookupSID("", "NT SERVICE\\TrustedInstaller")
	if err != nil {
		t.Fatal(err)
	}
	if !trustedSID(sid) {
		t.Fatal("Windows servicing SID was not recognized")
	}
}
