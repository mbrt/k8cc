package data

import (
	"fmt"
	"time"
)

// HostID is an identifier for a build host
type HostID int

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

// Lease represents a temporary assignment of hosts to a user
type Lease struct {
	User       User
	Expiration time.Time
	Hosts      []HostID
}

// TagLeases represents all the leases of a certain tag
type TagLeases struct {
	Tag    Tag
	Leases []Lease
}

// ScaleSettings contains scaling options
type ScaleSettings struct {
	MinReplicas     int
	MaxReplicas     int
	ReplicasPerUser int
	LeaseTime       time.Duration
}
