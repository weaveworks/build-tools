package main

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnauthorized = errors.New("unauthorized")

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
