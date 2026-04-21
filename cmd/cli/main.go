package main

import (
	"fmt"
	"os"

	"github.com/malcolm-getahead/local-mdm/cmd/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
