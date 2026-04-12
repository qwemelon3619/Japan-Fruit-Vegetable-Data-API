package main

import (
	"fmt"
	"os"

	"japan_data_project/internal/app/downloader"
)

func main() {
	if err := downloader.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
