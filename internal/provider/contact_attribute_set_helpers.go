package provider

import (
	"time"
)

// formatContactTimestamp renders an API timestamp as RFC3339. The typed client
// models carry optional timestamps as pointers; a missing one renders as the
// zero time, as it did when these responses were decoded into plain time.Time.
func formatContactTimestamp(t *time.Time) string {
	if t == nil {
		return time.Time{}.Format(time.RFC3339)
	}
	return t.Format(time.RFC3339)
}
