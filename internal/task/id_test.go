package task

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskID(t *testing.T) {
	id := NewTaskID()
	s := id.String()
	assert.NotEmpty(t, s, "expected non-empty string")
	assert.True(t, strings.HasPrefix(s, "tsk-"), "expected tsk- prefix, got %s", s)
	assert.Len(t, s, 9, "expected 9 characters (tsk- + 5 chars)")
}

func TestNewTaskID_Format(t *testing.T) {
	id := NewTaskID()
	s := id.String()
	// Verify format: tsk- followed by exactly 5 lowercase alphanumeric chars
	assert.Regexp(t, `^tsk-[a-z0-9]{5}$`, s)
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

func TestParseTaskID_ValidExamples(t *testing.T) {
	examples := []string{"tsk-abc12", "tsk-x7k2m", "tsk-00000", "tsk-zzzzz", "tsk-a1b2c"}
	for _, ex := range examples {
		parsed, err := ParseTaskID(ex)
		require.NoError(t, err, "expected valid for %q", ex)
		assert.Equal(t, ex, parsed.String())
	}
}

func TestParseTaskID_InvalidPrefix(t *testing.T) {
	_, err := ParseTaskID("repo-abc12")
	assert.Error(t, err, "expected error for wrong prefix")
}

func TestParseTaskID_OldFormat(t *testing.T) {
	_, err := ParseTaskID("tsk_01h2xcejqtf2nbrexx3vqjhp41")
	assert.Error(t, err, "expected error for old TypeID format")
}

func TestParseTaskID_EmptyString(t *testing.T) {
	_, err := ParseTaskID("")
	assert.Error(t, err, "expected error for empty string")
}

func TestParseTaskID_InvalidFormat(t *testing.T) {
	_, err := ParseTaskID("not-a-valid-id")
	assert.Error(t, err, "expected error for invalid format")
}

func TestParseTaskID_TooShort(t *testing.T) {
	_, err := ParseTaskID("tsk-abc")
	assert.Error(t, err, "expected error for too-short suffix")
}

func TestParseTaskID_TooLong(t *testing.T) {
	_, err := ParseTaskID("tsk-abcdef")
	assert.Error(t, err, "expected error for too-long suffix")
}

func TestParseTaskID_UpperCase(t *testing.T) {
	_, err := ParseTaskID("tsk-ABC12")
	assert.Error(t, err, "expected error for uppercase characters")
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

func TestTaskID_JSON(t *testing.T) {
	id := NewTaskID()

	data, err := json.Marshal(id)
	require.NoError(t, err)

	var parsed TaskID
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, id.String(), parsed.String())
}

func TestTaskID_IsZero(t *testing.T) {
	var zero TaskID
	assert.True(t, zero.IsZero())

	id := NewTaskID()
	assert.False(t, id.IsZero())
}
