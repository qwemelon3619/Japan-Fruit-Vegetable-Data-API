package main

import (
	"fmt"
	"os"

	"japan_data_project/internal/app/ingestor"
)

func main() {
	if err := ingestor.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
