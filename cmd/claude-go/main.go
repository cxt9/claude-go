package main

import (
	"fmt"
	"os"

	"github.com/cxt9/claude-go/internal/launcher"
)

func main() {
	if err := launcher.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
