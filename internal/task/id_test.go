package task

import (
	"strings"
	"testing"
)

func TestNewTaskID(t *testing.T) {
	id := NewTaskID()
	s := id.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
	if !strings.HasPrefix(s, "tsk_") {
		t.Errorf("expected tsk_ prefix, got %s", s)
	}
}

func TestNewTaskID_Unique(t *testing.T) {
	id1 := NewTaskID()
	id2 := NewTaskID()
	if id1.String() == id2.String() {
		t.Error("expected unique IDs, got identical values")
	}
}

func TestParseTaskID_Valid(t *testing.T) {
	original := NewTaskID()
	parsed, err := ParseTaskID(original.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.String() != original.String() {
		t.Errorf("expected %s, got %s", original.String(), parsed.String())
	}
}

func TestParseTaskID_InvalidPrefix(t *testing.T) {
	_, err := ParseTaskID("repo_01h2xcejqtf2nbrexx3vqjhp41")
	if err == nil {
		t.Error("expected error for wrong prefix")
	}
}

func TestParseTaskID_EmptyString(t *testing.T) {
	_, err := ParseTaskID("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestParseTaskID_InvalidFormat(t *testing.T) {
	_, err := ParseTaskID("not-a-valid-id")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestMustParseTaskID_Valid(t *testing.T) {
	original := NewTaskID()
	// Should not panic
	parsed := MustParseTaskID(original.String())
	if parsed.String() != original.String() {
		t.Errorf("expected %s, got %s", original.String(), parsed.String())
	}
}

func TestMustParseTaskID_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid task ID")
		}
	}()
	MustParseTaskID("invalid")
}
