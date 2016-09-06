package main

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnauthorized is the error when a user is unauthorized
var ErrUnauthorized = errors.New("unauthorized")

// ErrMultipleInstances is the error when a user has access to multiple
// instances, but we don't know which one to use.
type ErrMultipleInstances lookupView

func (e ErrMultipleInstances) Error() string {
	if len(e.Instances) == 0 {
		return "no available instances"
	}
	var instances []string
	for _, i := range e.Instances {
		instances = append(instances, fmt.Sprintf("%s (%s)", i.Name, i.ExternalID))
	}
	return fmt.Sprintf("multiple available instances: %s", strings.Join(instances, ", "))
}
