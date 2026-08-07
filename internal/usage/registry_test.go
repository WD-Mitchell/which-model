package usage

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	Register(Descriptor{ID: "t4-a", DisplayName: "A"})
	d, err := Get("t4-a")
	if err != nil || d.DisplayName != "A" {
		t.Fatalf("Get(t4-a) = (%v, %v), want ({A}, nil)", d, err)
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("t4-nope")
	var upe *UnknownProviderError
	if !errors.As(err, &upe) || upe.ID != "t4-nope" {
		t.Fatalf("Get(t4-nope) error = %v, want *UnknownProviderError{t4-nope}", err)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("duplicate Register did not panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, `duplicate provider id "t4-a"`) {
			t.Errorf("panic message = %v, want substring duplicate provider id \"t4-a\"", r)
		}
	}()
	Register(Descriptor{ID: "t4-a"})
}

func TestIDsContainsRegistered(t *testing.T) {
	if !slices.Contains(IDs(), "t4-a") {
		t.Errorf("IDs() = %v, want to contain t4-a", IDs())
	}
}

func TestAllContainsRegistered(t *testing.T) {
	found := false
	for _, d := range All() {
		if d.ID == "t4-a" {
			found = true
		}
	}
	if !found {
		t.Error("All() missing t4-a")
	}
}

func TestSortingAcrossRegistrationOrder(t *testing.T) {
	Register(Descriptor{ID: "t5-z", DisplayName: "Z"})
	Register(Descriptor{ID: "t5-m", DisplayName: "M"})
	Register(Descriptor{ID: "t5-a", DisplayName: "A"})

	ids := IDs()
	if !slices.IsSorted(ids) {
		t.Fatalf("IDs() not sorted: %v", ids)
	}
	for _, want := range []string{"t4-a", "t5-a", "t5-m", "t5-z"} {
		if !slices.Contains(ids, want) {
			t.Errorf("IDs() missing %q: %v", want, ids)
		}
	}
	ia := slices.Index(ids, "t5-a")
	im := slices.Index(ids, "t5-m")
	iz := slices.Index(ids, "t5-z")
	if !(ia < im && im < iz) {
		t.Errorf("sort order violated: a=%d m=%d z=%d in %v", ia, im, iz, ids)
	}

	all := All()
	if len(all) != len(ids) {
		t.Fatalf("len(All()) = %d, len(IDs()) = %d", len(all), len(ids))
	}
	for i := range all {
		if all[i].ID != ids[i] {
			t.Errorf("All()[%d].ID = %q, IDs()[%d] = %q", i, all[i].ID, i, ids[i])
		}
	}
	if all[0].ID != ids[0] {
		t.Errorf("All()[0].ID = %q, want the minimum registered ID %q", all[0].ID, ids[0])
	}
}
