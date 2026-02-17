package task

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskID(t *testing.T) {
	id := NewTaskID()
	s := id.String()
	assert.NotEmpty(t, s, "expected non-empty string")
	assert.True(t, strings.HasPrefix(s, "tsk_"), "expected tsk_ prefix, got %s", s)
}

func TestNewTaskID_Unique(t *testing.T) {
	id1 := NewTaskID()
	id2 := NewTaskID()
	assert.NotEqual(t, id1.String(), id2.String(), "expected unique IDs, got identical values")
}

func TestParseTaskID_Valid(t *testing.T) {
	original := NewTaskID()
	parsed, err := ParseTaskID(original.String())
	require.NoError(t, err)
	assert.Equal(t, original.String(), parsed.String())
}

func TestParseTaskID_InvalidPrefix(t *testing.T) {
	_, err := ParseTaskID("repo_01h2xcejqtf2nbrexx3vqjhp41")
	assert.Error(t, err, "expected error for wrong prefix")
}

func TestParseTaskID_EmptyString(t *testing.T) {
	_, err := ParseTaskID("")
	assert.Error(t, err, "expected error for empty string")
}

func TestParseTaskID_InvalidFormat(t *testing.T) {
	_, err := ParseTaskID("not-a-valid-id")
	assert.Error(t, err, "expected error for invalid format")
}

func TestMustParseTaskID_Valid(t *testing.T) {
	original := NewTaskID()
	// Should not panic
	parsed := MustParseTaskID(original.String())
	assert.Equal(t, original.String(), parsed.String())
}

func TestMustParseTaskID_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustParseTaskID("invalid")
	}, "expected panic for invalid task ID")
}
