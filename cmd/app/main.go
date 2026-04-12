package main

import (
	"fmt"
	"os"

	"japan_data_project/internal/app/runner"
)

func main() {
	if err := runner.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
