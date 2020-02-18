package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/concourse/baggageclaim/cmd/beltloader/pkg"
	"github.com/jessevdk/go-flags"
)

type BeltloaderCommand struct {
	// Uris indicates where to pull volumes from, and where to place them.
	// Format: `src=https://github.com/blabla,dst=/tmp/blabla`
	//
	Uris []string `long:"uri" required:"true" description:"uris to stream from"`
}

func run(cmd *BeltloaderCommand) error {
	for _, uri := range cmd.Uris {
		rv, err := pkg.NewRemoteVolume(uri)
		if err != nil {
			return fmt.Errorf("new remote vol: %w", err)
		}

		err = rv.Pull(context.Background())
		if err != nil {
			return fmt.Errorf("pull remote vol: %w", err)
		}
	}

	return nil
}

func main() {
	cmd := &BeltloaderCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	err = run(cmd)
	if err != nil {
		log.Fatal(err)
	}

}
