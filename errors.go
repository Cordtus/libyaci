package libyaci

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnsupportedMethod is returned when the connected server does not expose a
// requested method through reflection.
var ErrUnsupportedMethod = errors.New("unsupported method")

// UnsupportedMethodError describes a method that is not available on the
// connected server according to reflected descriptors.
type UnsupportedMethodError struct {
	Method            string
	Service           string
	SDKVersion        string
	Reason            string
	Available         []string
	AvailableServices []string
}

func (e *UnsupportedMethodError) Error() string {
	var b strings.Builder
	if e.Method != "" {
		fmt.Fprintf(&b, "%s: %s", ErrUnsupportedMethod, e.Method)
	} else {
		b.WriteString(ErrUnsupportedMethod.Error())
	}
	if e.SDKVersion != "" {
		fmt.Fprintf(&b, " (configured SDK %s)", e.SDKVersion)
	}
	if e.Reason != "" {
		b.WriteString(": ")
		b.WriteString(e.Reason)
	}
	if len(e.Available) > 0 {
		b.WriteString("; available methods: ")
		b.WriteString(strings.Join(e.Available, ", "))
	}
	if len(e.AvailableServices) > 0 {
		b.WriteString("; similar advertised services: ")
		b.WriteString(strings.Join(e.AvailableServices, ", "))
	}
	return b.String()
}

func (e *UnsupportedMethodError) Unwrap() error {
	return ErrUnsupportedMethod
}
