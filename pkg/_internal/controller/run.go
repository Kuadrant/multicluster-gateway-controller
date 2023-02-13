package controller

import (
	"errors"
	"os"
)

// IsRunningLocally checks whether the controller is running locally or as a
// container. It does it by checking that the binary `/controller` exists. If it
// does, it's considered to be running in a container, and returns `false`.
// Returns `true` otherwise
func IsRunningLocally() bool {
	_, err := os.Stat("/controller")
	return errors.Is(err, os.ErrNotExist)
}
