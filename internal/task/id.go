package task

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
)

const (
	taskIDPrefix  = "tsk-"
	taskIDSuffLen = 5
	taskIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// taskIDPattern matches a valid task ID: tsk- followed by exactly 5 lowercase alphanumeric characters.
var taskIDPattern = regexp.MustCompile(`^tsk-[a-z0-9]{5}$`)

// TaskID is the unique identifier for a Task.
// Format: tsk-<5 lowercase alphanumeric chars>, e.g. tsk-a1b2c
type TaskID struct {
	val string
}

// NewTaskID generates a new unique TaskID.
func NewTaskID() TaskID {
	b := make([]byte, taskIDSuffLen)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("task: failed to generate random ID: %v", err))
	}
	for i := range b {
		b[i] = taskIDAlphabet[int(b[i])%len(taskIDAlphabet)]
	}
	return TaskID{val: taskIDPrefix + string(b)}
}

// ParseTaskID parses a string into a TaskID.
func ParseTaskID(s string) (TaskID, error) {
	if !taskIDPattern.MatchString(s) {
		return TaskID{}, fmt.Errorf("invalid task ID %q: must match tsk-[a-z0-9]{5}", s)
	}
	return TaskID{val: s}, nil
}

// MustParseTaskID parses a string into a TaskID, panicking on failure.
func MustParseTaskID(s string) TaskID {
	id, err := ParseTaskID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string representation of the TaskID.
func (id TaskID) String() string {
	return id.val
}

// IsZero returns true if the TaskID is the zero value.
func (id TaskID) IsZero() bool {
	return id.val == ""
}

// MarshalJSON implements json.Marshaler.
func (id TaskID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.val)
}

// UnmarshalJSON implements json.Unmarshaler.
func (id *TaskID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*id = TaskID{}
		return nil
	}
	parsed, err := ParseTaskID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
