// Package securestore provides narrow native OS credential operations. It knows
// nothing about providers, mutable configuration or company authorization.
package securestore

type Kind string

const CatalogService = "which-model-catalog"
const CatalogAccount = "artificial-analysis"
const CatalogLegacyFile = "aa_api_key"

const (
	Missing     Kind = "missing"
	Locked      Kind = "locked"
	Denied      Kind = "denied"
	Unavailable Kind = "unavailable"
	TooLarge    Kind = "too_large"
)

// Error deliberately retains no underlying OS text, command output or secret.
type Error struct{ Kind Kind }

func (e *Error) Error() string {
	switch e.Kind {
	case Missing:
		return "credential was not found in the OS secure store"
	case Locked:
		return "OS secure store is locked or requires interaction; unlock it and retry"
	case Denied:
		return "OS secure store access was denied"
	case TooLarge:
		return "credential exceeds the native secure-store size limit"
	default:
		return "OS secure store is unavailable"
	}
}

func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && other.Kind == e.Kind
}

type Store interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

type unavailableStore struct{}

func (unavailableStore) Get(string, string) (string, error) { return "", &Error{Unavailable} }
func (unavailableStore) Set(string, string, string) error   { return &Error{Unavailable} }
func (unavailableStore) Delete(string, string) error        { return &Error{Unavailable} }
