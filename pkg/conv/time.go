package conv

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ToKubeTime converts correctly a standard time to a kubernetes time.
//
// It's necessary to truncate nanoseconds to allow correct comparison.
func ToKubeTime(t time.Time) *metav1.Time {
	r := metav1.NewTime(t).Rfc3339Copy()
	return &r
}
