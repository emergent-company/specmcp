package main

import (
	"fmt"
)

// runVersion handles the "specmcp version" subcommand.
func runVersion() {
	fmt.Printf("specmcp version %s\n", Version)
}
