//go:build nousage

package offlinedesktop

import (
	"bytes"
	"encoding/json"
	"github.com/WD-Mitchell/which-model/pkg/scoreonly"
	"reflect"
	"testing"
)

func TestOfflineDesktopMatchesRestrictedRanking(t *testing.T) {
	a := &API{}
	profiles, e := a.Profiles()
	if e != nil || len(profiles) == 0 {
		t.Fatal(e)
	}
	for _, p := range profiles {
		got, e := a.Rank(p, 3)
		if e != nil {
			t.Fatal(e)
		}
		var out, err bytes.Buffer
		if scoreonly.Run([]string{"pick", "--profile", p, "--top", "3", "--json"}, &out, &err) != 0 {
			t.Fatal(err.String())
		}
		if !bytes.Equal(got, out.Bytes()) {
			t.Fatal("desktop ranking changed")
		}
	}
}
func TestOfflineDesktopHasOnlyReadOnlyBindings(t *testing.T) {
	typ := reflect.TypeOf(&API{})
	if typ.NumMethod() != 3 {
		t.Fatal("unexpected bound operation")
	}
	for _, name := range []string{"Rank", "Profiles", "Capabilities"} {
		if _, ok := typ.MethodByName(name); !ok {
			t.Fatal(name)
		}
	}
	if _, err := (&API{}).Rank("../../config", 3); err == nil {
		t.Fatal("accepted non-builtin profile")
	}
	raw, e := (&API{}).Capabilities()
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	if m["artifact"] != "which-model-score-only-desktop" || m["runtime_boundary"] == nil {
		t.Fatal("wrong host boundary")
	}
}
