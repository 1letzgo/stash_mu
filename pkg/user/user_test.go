package user

import (
	"context"
	"reflect"
	"testing"
)

func TestRoleAtLeast(t *testing.T) {
	tests := []struct {
		r, o Role
		want bool
	}{
		{RoleAdmin, RoleViewer, true},
		{RoleAdmin, RoleEditor, true},
		{RoleAdmin, RoleAdmin, true},
		{RoleEditor, RoleAdmin, false},
		{RoleEditor, RoleEditor, true},
		{RoleViewer, RoleEditor, false},
		{Role("bogus"), RoleViewer, false},
	}
	for _, tt := range tests {
		if got := tt.r.AtLeast(tt.o); got != tt.want {
			t.Errorf("%s.AtLeast(%s) = %v, want %v", tt.r, tt.o, got, tt.want)
		}
	}
}

func TestParseRole(t *testing.T) {
	if r, err := ParseRole(" editor "); err != nil || r != RoleEditor {
		t.Errorf("ParseRole(editor) = %v, %v", r, err)
	}
	if _, err := ParseRole("root"); err == nil {
		t.Error("expected error for unknown role")
	}
}

func TestCurrent(t *testing.T) {
	ctx := context.Background()
	if got := Current(ctx); !reflect.DeepEqual(got, System) {
		t.Errorf("Current(empty) = %v, want System", got)
	}
	u := User{ID: 5, Username: "bob", Role: RoleViewer}
	ctx = WithUser(ctx, u)
	if got := Current(ctx); !reflect.DeepEqual(got, u) {
		t.Errorf("Current = %v, want %v", got, u)
	}
	if CurrentID(ctx) != 5 {
		t.Error("CurrentID mismatch")
	}
}

func TestLibraryRestricted(t *testing.T) {
	if (User{Role: RoleViewer}).LibraryRestricted() {
		t.Error("viewer without libraries must not be restricted")
	}
	if !(User{Role: RoleEditor, Libraries: []string{"/a"}}).LibraryRestricted() {
		t.Error("editor with libraries must be restricted")
	}
	if (User{Role: RoleAdmin, Libraries: []string{"/a"}}).LibraryRestricted() {
		t.Error("admin must never be restricted")
	}
}
