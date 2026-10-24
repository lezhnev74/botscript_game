// Command replay reruns a Record (JSON) or a scenario (.scn), prints the hash
// of its event log, optionally checks it against an expected hash, and with
// -export writes the exact WebSocket message sequence (a snapshot, then one
// beat message per beat) as a UI fixture.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "replay:", err)
		os.Exit(1)
	}
}
