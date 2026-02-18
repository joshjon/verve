package epic

import (
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type epicPrefix struct{}

func (epicPrefix) Prefix() string { return "epc" }

// EpicID is the unique identifier for an Epic.
type EpicID struct {
	typeid.TypeID[epicPrefix]
}

// NewEpicID generates a new unique EpicID.
func NewEpicID() EpicID {
	return id.New[EpicID]()
}

// ParseEpicID parses a string into an EpicID.
func ParseEpicID(s string) (EpicID, error) {
	return id.Parse[EpicID](s)
}

// MustParseEpicID parses a string into an EpicID, panicking on failure.
func MustParseEpicID(s string) EpicID {
	return id.MustParse[EpicID](s)
}
