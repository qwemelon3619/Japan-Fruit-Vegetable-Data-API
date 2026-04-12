package runner

import (
	"errors"
	"fmt"

	appapi "japan_data_project/internal/app/api"
	"japan_data_project/internal/app/downloader"
	"japan_data_project/internal/app/ingestor"
)

func Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "api":
		return appapi.Run()
	case "download", "downloader":
		return downloader.Run(rest)
	case "ingestor", "ingest":
		return ingestor.Run(rest)
	case "help", "-h", "--help":
		return usageError()
	default:
		return fmt.Errorf("unknown subcommand: %s\n\n%s", sub, usageMessage())
	}
}

func usageError() error {
	return errors.New(usageMessage())
}

func usageMessage() string {
	return "usage: app <api|download|ingestor> [options]\n" +
		"  app api\n" +
		"  app download -date 20260407 -out ./data/data_downloads\n" +
		"  app ingestor"
}
