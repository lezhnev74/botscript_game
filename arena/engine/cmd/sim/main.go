// Command sim runs matches headless and prints stats (kills per beat, beats to
// the first hit, coins, ...) for tuning the rules.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "sim:", err)
		os.Exit(1)
	}
}
