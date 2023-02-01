package controller

import (
	"errors"
	"os"
)

var namespace string = ""

// IsRunningLocally checks whether the controller is running locally or as a
// container. It does it by checking that the binary `/manager` exists. If it
// does, it's considered to be running in a container, and returns `false`.
// Returns `true` otherwise
func IsRunningLocally() bool {
	_, err := os.Stat("/manager")
	return errors.Is(err, os.ErrNotExist)
}

// GetNamespace returns the namespace where the controller is running. If the
// controller is running in the local host, and the --namespace flag has not been
// passed, returns false
func GetNamespace() (string, bool) {
	return namespace, namespace != ""
}

func FindNamespace() (string, bool) {
	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", false
	}

	return string(ns), true
}

// SpecifiedNamespace sets the controller namespace to ns, if and only if the
// namespace could not be calculated automatically
func SpecifiedNamespace(ns string) {
	if namespace == "" {
		namespace = ns
	}
}

func init() {
	namespace, _ = FindNamespace()
}
