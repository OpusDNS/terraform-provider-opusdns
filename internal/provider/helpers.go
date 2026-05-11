package provider

import (
	"errors"

	"github.com/opusdns/opusdns-go-client/opusdns"
)

// isNotFound returns true when an API error indicates a 404 Not Found response.
func isNotFound(err error) bool {
	return errors.Is(err, opusdns.ErrNotFound)
}
