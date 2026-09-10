//go:build linux && !nousage

package securestore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	secretName       = "org.freedesktop.secrets"
	secretService    = "org.freedesktop.Secret.Service"
	secretCollection = "org.freedesktop.Secret.Collection"
	secretItem       = "org.freedesktop.Secret.Item"
)

// socket is a private native-test seam. Production uses the fixed local user
// runtime socket and ignores DBUS_SESSION_BUS_ADDRESS and XDG_RUNTIME_DIR.
type linuxStore struct{ socket string }

func Native() Store { return linuxStore{} }

func classifyLinux(err error) error {
	if err == nil {
		return nil
	}
	var own *Error
	if errors.As(err, &own) {
		return own
	}
	var remote dbus.Error
	var pointer *dbus.Error
	name := ""
	if errors.As(err, &remote) {
		name = remote.Name
	} else if errors.As(err, &pointer) {
		name = pointer.Name
	}
	if name != "" {
		switch name {
		case "org.freedesktop.Secret.Error.IsLocked":
			return &Error{Locked}
		case "org.freedesktop.DBus.Error.AccessDenied", "org.freedesktop.DBus.Error.AuthFailed", "org.freedesktop.Secret.Error.Denied":
			return &Error{Denied}
		case "org.freedesktop.Secret.Error.NoSuchObject":
			return &Error{Missing}
		}
	}
	return &Error{Unavailable}
}

func (s linuxStore) connect() (context.Context, *dbus.Conn, func(), error) {
	path := s.socket
	if path == "" {
		path = filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "bus")
	}
	for _, entry := range []struct {
		path      string
		directory bool
	}{{filepath.Dir(path), true}, {path, false}} {
		info, err := os.Lstat(entry.path)
		if err != nil {
			return nil, nil, nil, &Error{Unavailable}
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Getuid()) || info.Mode()&os.ModeSymlink != 0 {
			return nil, nil, nil, &Error{Denied}
		}
		if entry.directory {
			if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
				return nil, nil, nil, &Error{Denied}
			}
		} else if info.Mode()&os.ModeSocket == 0 {
			return nil, nil, nil, &Error{Denied}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := dbus.Connect("unix:path="+path, dbus.WithContext(ctx), dbus.WithAuth(dbus.AuthExternal(strconv.Itoa(os.Getuid()))))
	if err != nil {
		cancel()
		return nil, nil, nil, classifyLinux(err)
	}
	return ctx, conn, func() { conn.Close(); cancel() }, nil
}

func property(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath, iface, name string) (dbus.Variant, error) {
	var result dbus.Variant
	err := conn.Object(secretName, path).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, iface, name).Store(&result)
	return result, classifyLinux(err)
}

func collection(ctx context.Context, conn *dbus.Conn) (dbus.ObjectPath, error) {
	var path dbus.ObjectPath
	err := conn.Object(secretName, "/org/freedesktop/secrets").CallWithContext(ctx, secretService+".ReadAlias", 0, "default").Store(&path)
	if err != nil {
		return "", classifyLinux(err)
	}
	if path == "/" || !path.IsValid() {
		return "", &Error{Unavailable}
	}
	locked, err := property(ctx, conn, path, secretCollection, "Locked")
	if err != nil {
		return "", err
	}
	value, ok := locked.Value().(bool)
	if !ok {
		return "", &Error{Unavailable}
	}
	if value {
		return "", &Error{Locked}
	}
	return path, nil
}

func findItem(ctx context.Context, conn *dbus.Conn, service, account string) (dbus.ObjectPath, error) {
	// Check the collection before searching: an empty locked store is locked,
	// not a missing item that could incorrectly authorize a fallback.
	if _, err := collection(ctx, conn); err != nil {
		return "", err
	}
	var unlocked, locked []dbus.ObjectPath
	err := conn.Object(secretName, "/org/freedesktop/secrets").CallWithContext(ctx, secretService+".SearchItems", 0, map[string]string{"service": service, "username": account}).Store(&unlocked, &locked)
	if err != nil {
		return "", classifyLinux(err)
	}
	if len(locked) > 0 {
		return "", &Error{Locked}
	}
	if len(unlocked) == 0 {
		return "", &Error{Missing}
	}
	if len(unlocked) != 1 || !unlocked[0].IsValid() {
		return "", &Error{Unavailable}
	}
	return unlocked[0], nil
}

func openSession(ctx context.Context, conn *dbus.Conn) (dbus.ObjectPath, error) {
	var output dbus.Variant
	var path dbus.ObjectPath
	err := conn.Object(secretName, "/org/freedesktop/secrets").CallWithContext(ctx, secretService+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&output, &path)
	if err != nil {
		return "", classifyLinux(err)
	}
	if path == "/" || !path.IsValid() {
		return "", &Error{Unavailable}
	}
	return path, nil
}

type linuxSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func (s linuxStore) Get(service, account string) (string, error) {
	ctx, conn, close, err := s.connect()
	if err != nil {
		return "", err
	}
	defer close()
	item, err := findItem(ctx, conn, service, account)
	if err != nil {
		return "", err
	}
	session, err := openSession(ctx, conn)
	if err != nil {
		return "", err
	}
	defer conn.Object(secretName, session).CallWithContext(ctx, "org.freedesktop.Secret.Session.Close", 0)
	var secret linuxSecret
	err = conn.Object(secretName, item).CallWithContext(ctx, secretItem+".GetSecret", 0, session).Store(&secret)
	if err != nil {
		return "", classifyLinux(err)
	}
	if secret.Session != session || len(secret.Parameters) != 0 || len(secret.Value) > 64*1024 {
		return "", &Error{Unavailable}
	}
	return string(secret.Value), nil
}

func dismissPrompt(ctx context.Context, conn *dbus.Conn, prompt dbus.ObjectPath) error {
	if prompt == "/" {
		return nil
	}
	if prompt.IsValid() {
		conn.Object(secretName, prompt).CallWithContext(ctx, "org.freedesktop.Secret.Prompt.Dismiss", 0)
	}
	return &Error{Locked}
}

func (s linuxStore) Set(service, account, value string) error {
	if len(value) > 64*1024 {
		return &Error{TooLarge}
	}
	ctx, conn, close, err := s.connect()
	if err != nil {
		return err
	}
	defer close()
	col, err := collection(ctx, conn)
	if err != nil {
		return err
	}
	session, err := openSession(ctx, conn)
	if err != nil {
		return err
	}
	defer conn.Object(secretName, session).CallWithContext(ctx, "org.freedesktop.Secret.Session.Close", 0)
	properties := map[string]dbus.Variant{
		secretItem + ".Label":      dbus.MakeVariant("which-model credential"),
		secretItem + ".Attributes": dbus.MakeVariant(map[string]string{"service": service, "username": account}),
	}
	var item, prompt dbus.ObjectPath
	err = conn.Object(secretName, col).CallWithContext(ctx, secretCollection+".CreateItem", 0, properties, linuxSecret{session, []byte{}, []byte(value), "text/plain; charset=utf-8"}, true).Store(&item, &prompt)
	if err != nil {
		return classifyLinux(err)
	}
	if err := dismissPrompt(ctx, conn, prompt); err != nil {
		return err
	}
	if item == "/" || !item.IsValid() {
		return &Error{Unavailable}
	}
	return nil
}

func (s linuxStore) Delete(service, account string) error {
	ctx, conn, close, err := s.connect()
	if err != nil {
		return err
	}
	defer close()
	item, err := findItem(ctx, conn, service, account)
	if err != nil {
		return err
	}
	var prompt dbus.ObjectPath
	err = conn.Object(secretName, item).CallWithContext(ctx, secretItem+".Delete", 0).Store(&prompt)
	if err != nil {
		return classifyLinux(err)
	}
	return dismissPrompt(ctx, conn, prompt)
}
