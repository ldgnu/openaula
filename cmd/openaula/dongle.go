package main

import "github.com/ldgnu/openaula/internal/aula"

// dongleOpen is a small indirection so watch can query the battery.
func dongleOpen() (*aula.Dongle, error) { return aula.OpenDongle() }
