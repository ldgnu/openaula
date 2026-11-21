//go:build !linux && !windows

package aula

import "fmt"

// openTransport is only implemented for Linux and Windows.
func openTransport() (Transport, error) {
	return nil, fmt.Errorf("esta plataforma no está soportada todavía")
}
