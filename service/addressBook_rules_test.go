package service

import (
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

func rulePtr(rule int) *int {
	return &rule
}

func TestResolveRuleByPriority(t *testing.T) {
	tests := []struct {
		name      string
		inherited int
		personal  *int
		group     *int
		want      int
	}{
		{
			name:      "inherits when no override exists",
			inherited: model.ShareAddressBookRuleRuleReadWrite,
			want:      model.ShareAddressBookRuleRuleReadWrite,
		},
		{
			name:      "uses group when personal is absent",
			inherited: model.ShareAddressBookRuleRuleNone,
			group:     rulePtr(model.ShareAddressBookRuleRuleReadWrite),
			want:      model.ShareAddressBookRuleRuleReadWrite,
		},
		{
			name:      "personal overrides group",
			inherited: model.ShareAddressBookRuleRuleRead,
			personal:  rulePtr(model.ShareAddressBookRuleRuleRead),
			group:     rulePtr(model.ShareAddressBookRuleRuleFullControl),
			want:      model.ShareAddressBookRuleRuleRead,
		},
		{
			name:      "explicit personal none overrides group",
			inherited: model.ShareAddressBookRuleRuleReadWrite,
			personal:  rulePtr(model.ShareAddressBookRuleRuleNone),
			group:     rulePtr(model.ShareAddressBookRuleRuleReadWrite),
			want:      model.ShareAddressBookRuleRuleNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveRuleByPriority(tt.inherited, tt.personal, tt.group)
			if got != tt.want {
				t.Fatalf("ResolveRuleByPriority() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolveOwnedRule(t *testing.T) {
	got := ResolveOwnedRule(10, 10, model.ShareAddressBookRuleRuleNone, rulePtr(model.ShareAddressBookRuleRuleNone), rulePtr(model.ShareAddressBookRuleRuleNone))
	if got != model.ShareAddressBookRuleRuleFullControl {
		t.Fatalf("ResolveOwnedRule() = %d, want owner full control", got)
	}
}

func TestResolveAddressBookRule(t *testing.T) {
	rowID := uint(42)
	if got := ResolveAddressBookRule(model.ShareAddressBookRuleRuleReadWrite, rowID, nil, nil); got != model.ShareAddressBookRuleRuleReadWrite {
		t.Fatalf("inherited record rule = %d, want read/write", got)
	}

	personal := map[uint]int{rowID: model.ShareAddressBookRuleRuleNone}
	group := map[uint]int{rowID: model.ShareAddressBookRuleRuleReadWrite}
	if got := ResolveAddressBookRule(model.ShareAddressBookRuleRuleReadWrite, rowID, personal, group); got != model.ShareAddressBookRuleRuleNone {
		t.Fatalf("personal none record rule = %d, want none", got)
	}

	if got := ResolveAddressBookRule(model.ShareAddressBookRuleRuleNone, rowID, nil, group); got != model.ShareAddressBookRuleRuleReadWrite {
		t.Fatalf("group fallback record rule = %d, want read/write", got)
	}
}
