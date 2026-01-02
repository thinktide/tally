package main

import "github.com/thinktide/tally/internal/cli"

// main is the entry point of the application.
//
// It delegates execution to the [cli.Execute] function which initializes and runs the CLI commands.
// Any errors encountered during command execution result in the application exiting with a non-zero code.
//
// # Thread Safety
//
// This function is safe for concurrent use as it orchestrates a single-threaded CLI execution flow.
func main() {
	cli.Execute()
}
