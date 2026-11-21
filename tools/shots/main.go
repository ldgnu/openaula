// Command shots prints a TUI screen with demo data, for documentation images.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/ldgnu/openaula/internal/tui"
)

func main() {
	name := "menu"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	frame := 0
	if len(os.Args) > 2 {
		frame, _ = strconv.Atoi(os.Args[2])
	}
	fmt.Print(tui.DocShot(name, frame))
}
