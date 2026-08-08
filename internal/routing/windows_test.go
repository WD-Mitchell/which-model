package routing

import (
	"reflect"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestBindWindowIDs(t *testing.T) {
	cases := []struct {
		name     string
		windows  []usage.WindowSpec
		modelID  string
		model    string
		want     []string
	}{
		{
			name:    "account windows",
			windows: []usage.WindowSpec{{ID: "5h"}, {ID: "weekly"}},
			want:    []string{"5h", "weekly"},
		},
		{
			name:    "opus scope",
			windows: []usage.WindowSpec{{ID: "5h"}, {ID: "opus_7d", ModelScope: []string{"opus"}}},
			modelID: "claude-opus-4-5-20251101",
			want:    []string{"5h", "opus_7d"},
		},
		{
			name:    "unmatched scope",
			windows: []usage.WindowSpec{{ID: "5h"}, {ID: "sonnet_7d", ModelScope: []string{"sonnet"}}},
			modelID: "claude-opus-4-5-20251101",
			want:    []string{"5h"},
		},
		{
			name:    "model substring",
			windows: []usage.WindowSpec{{ID: "opus_7d", ModelScope: []string{"opus 5"}}},
			model:   "Claude Opus 5",
			want:    []string{"opus_7d"},
		},
		{
			name:    "case insensitive",
			windows: []usage.WindowSpec{{ID: "opus_7d", ModelScope: []string{"OPUS"}}},
			modelID: "claude-opus-4-5-20251101",
			want:    []string{"opus_7d"},
		},
		{
			name:    "exact model id",
			windows: []usage.WindowSpec{{ID: "opus_7d", ModelScope: []string{"claude-opus-4-5-20251101"}}},
			modelID: "claude-opus-4-5-20251101",
			want:    []string{"opus_7d"},
		},
		{
			name:    "all scoped matches",
			windows: []usage.WindowSpec{{ID: "a", ModelScope: []string{"opus"}}, {ID: "b", ModelScope: []string{"opus"}}},
			modelID: "claude-opus-4-5-20251101",
			want:    []string{"a", "b"},
		},
		{
			name:    "account first",
			windows: []usage.WindowSpec{{ID: "opus_7d", ModelScope: []string{"opus"}}, {ID: "5h"}},
			modelID: "claude-opus-4-5-20251101",
			want:    []string{"5h", "opus_7d"},
		},
		{
			name:    "deduplicate account windows",
			windows: []usage.WindowSpec{{ID: "5h"}, {ID: "5h"}},
			want:    []string{"5h"},
		},
		{
			name:    "empty",
			windows: []usage.WindowSpec{},
			want:    []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BindWindowIDs(tc.windows, tc.modelID, tc.model)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("BindWindowIDs() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
