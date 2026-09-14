//go:build linux && !nousage

package securestore

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type protocolSecretService struct {
	mu              sync.Mutex
	conn            *dbus.Conn
	current         dbus.ObjectPath
	corruptReadback bool
	lockedMatches   bool
	writes          int
	items           map[dbus.ObjectPath]*protocolSecretItem
}
type protocolSecretItem struct {
	service *protocolSecretService
	path    dbus.ObjectPath
	secret  linuxSecret
}
type protocolSecretCollection struct {
	service *protocolSecretService
	path    dbus.ObjectPath
}
type protocolSecretSession struct{}

func (*protocolSecretSession) Close() *dbus.Error { return nil }
func (s *protocolSecretService) ReadAlias(string) (dbus.ObjectPath, *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current, nil
}
func (s *protocolSecretService) OpenSession(string, dbus.Variant) (dbus.Variant, dbus.ObjectPath, *dbus.Error) {
	return dbus.MakeVariant(""), "/protocol/session", nil
}
func (s *protocolSecretService) SearchItems(map[string]string) ([]dbus.ObjectPath, []dbus.ObjectPath, *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := []dbus.ObjectPath{}
	// The fixture has exactly one service/account pair, shared by both collections.
	for path := range s.items {
		found = append(found, path)
	}
	if s.lockedMatches {
		return []dbus.ObjectPath{}, found, nil
	}
	return found, []dbus.ObjectPath{}, nil
}
func (*protocolSecretCollection) Get(string, string) (dbus.Variant, *dbus.Error) {
	return dbus.MakeVariant(false), nil
}
func (c *protocolSecretCollection) CreateItem(_ map[string]dbus.Variant, secret linuxSecret, replace bool) (dbus.ObjectPath, dbus.ObjectPath, *dbus.Error) {
	c.service.mu.Lock()
	defer c.service.mu.Unlock()
	c.service.writes++
	path := dbus.ObjectPath(string(c.path) + "/item")
	// CreateItem replaces matching attributes only within its own collection.
	if item, ok := c.service.items[path]; ok && replace {
		item.secret = secret
	} else {
		item := &protocolSecretItem{service: c.service, path: path, secret: secret}
		c.service.items[path] = item
		if err := c.service.conn.Export(item, path, secretItem); err != nil {
			panic(err)
		}
	}
	return path, "/", nil
}
func (i *protocolSecretItem) GetSecret(session dbus.ObjectPath) (linuxSecret, *dbus.Error) {
	i.service.mu.Lock()
	defer i.service.mu.Unlock()
	secret := i.secret
	secret.Session = session
	if i.service.corruptReadback {
		secret.Value = []byte("SYNTHETIC_DIFFERENT_VALUE")
	}
	return secret, nil
}
func (i *protocolSecretItem) SetSecret(secret linuxSecret) *dbus.Error {
	i.service.mu.Lock()
	defer i.service.mu.Unlock()
	i.service.writes++
	i.secret = secret
	return nil
}
func (i *protocolSecretItem) Delete() (dbus.ObjectPath, *dbus.Error) {
	i.service.mu.Lock()
	defer i.service.mu.Unlock()
	delete(i.service.items, i.path)
	return "/", nil
}

func protocolStore(t *testing.T) (linuxStore, *protocolSecretService) {
	t.Helper()
	dir, err := os.MkdirTemp("", "wm-ss-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "bus")
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		if os.Getenv("WHICH_MODEL_NATIVE_STORE_TEST") == "1" {
			t.Fatal("native CI requires dbus-daemon")
		}
		t.Skip("protocol fixture requires dbus-daemon")
	}
	bus := exec.Command(daemon, "--session", "--nofork", "--nopidfile", "--address=unix:path="+socket)
	if err := bus.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Process.Kill(); _ = bus.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("test bus failed to start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	conn, err := dbus.Connect("unix:path=" + socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	svc := &protocolSecretService{conn: conn, current: "/protocol/first", items: map[dbus.ObjectPath]*protocolSecretItem{}}
	if err := conn.Export(svc, "/org/freedesktop/secrets", secretService); err != nil {
		t.Fatal(err)
	}
	if err := conn.Export(&protocolSecretSession{}, "/protocol/session", "org.freedesktop.Secret.Session"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []dbus.ObjectPath{"/protocol/first", "/protocol/second"} {
		col := &protocolSecretCollection{service: svc, path: path}
		if err := conn.Export(col, path, secretCollection); err != nil {
			t.Fatal(err)
		}
		if err := conn.Export(col, path, "org.freedesktop.DBus.Properties"); err != nil {
			t.Fatal(err)
		}
	}
	reply, err := conn.RequestName(secretName, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("fixture name: %v %v", reply, err)
	}
	return linuxStore{socket: socket}, svc
}

func TestLinuxUpdateAfterDefaultCollectionChanges(t *testing.T) {
	store, svc := protocolStore(t)
	for _, value := range []string{"SYNTHETIC_FIRST", "SYNTHETIC_UPDATED"} {
		if err := store.Set("which-model-protocol", "account", value); err != nil {
			t.Fatal(err)
		}
		if got, err := store.Get("which-model-protocol", "account"); err != nil || got != value {
			t.Fatalf("same-collection round trip failed: %v", err)
		}
	}
	svc.mu.Lock()
	svc.current = "/protocol/second"
	svc.mu.Unlock()
	// Both collections remain unlocked, as when a user chooses a new default wallet.
	if _, err := store.Get("which-model-protocol", "account"); err != nil {
		t.Fatalf("old entry should remain readable before update: %v", err)
	}
	if err := store.Set("which-model-protocol", "account", "SYNTHETIC_NEW_DEFAULT"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	got, readErr := store.Get("which-model-protocol", "account")
	svc.mu.Lock()
	count := len(svc.items)
	svc.mu.Unlock()
	if readErr != nil || got != "SYNTHETIC_NEW_DEFAULT" || count != 1 {
		t.Fatalf("successful update across defaults stranded credential: items=%d Get=%v", count, readErr)
	}

	if err := store.Delete("which-model-protocol", "account"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("which-model-protocol", "account"); !errors.Is(err, &Error{Missing}) {
		t.Fatalf("delete left a credential: %v", err)
	}
}

func TestLinuxWriteRequiresExactReadback(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "create"
		if existing {
			name = "update"
		}
		t.Run(name, func(t *testing.T) {
			store, svc := protocolStore(t)
			if existing {
				if err := store.Set("which-model-protocol", "account", "SYNTHETIC_BEFORE"); err != nil {
					t.Fatal(err)
				}
			}
			svc.mu.Lock()
			svc.corruptReadback = true
			svc.mu.Unlock()
			if err := store.Set("which-model-protocol", "account", "SYNTHETIC_AFTER"); !errors.Is(err, &Error{Unavailable}) {
				t.Fatalf("unverified write returned success: %v", err)
			}
		})
	}
}

func TestLinuxWriteRefusesAmbiguousOrLockedMatches(t *testing.T) {
	for _, name := range []string{"ambiguous", "locked"} {
		t.Run(name, func(t *testing.T) {
			store, svc := protocolStore(t)
			if err := store.Set("which-model-protocol", "account", "SYNTHETIC_BEFORE"); err != nil {
				t.Fatal(err)
			}
			svc.mu.Lock()
			want := Unavailable
			if name == "locked" {
				svc.lockedMatches = true
				want = Locked
			} else {
				// A second independently existing match must not be overwritten.
				svc.items["/protocol/second/item"] = &protocolSecretItem{}
			}
			before := svc.writes
			svc.mu.Unlock()
			err := store.Set("which-model-protocol", "account", "SYNTHETIC_AFTER")
			svc.mu.Lock()
			writes := svc.writes - before
			svc.mu.Unlock()
			if !errors.Is(err, &Error{want}) || writes != 0 {
				t.Fatalf("unsafe match selection: err=%v writes=%d", err, writes)
			}
		})
	}
}
