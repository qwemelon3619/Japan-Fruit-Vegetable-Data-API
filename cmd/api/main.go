package main

import (
	"fmt"
	"os"

	appapi "japan_data_project/internal/app/api"
)

func main() {
	if err := appapi.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
