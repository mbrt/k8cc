package data

import (
	"fmt"
)

// Tag is an identifier for a certain build environment
type Tag struct {
	Namespace string
	Name      string
}

func (t Tag) String() string {
	return fmt.Sprintf("%s/%s", t.Namespace, t.Name)
}

// User is an identifier for a user
type User string
