// One-shot HotSpot Attach client; no service management or firmware writes.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "REFUSED:", err)
		os.Exit(1)
	}
}
