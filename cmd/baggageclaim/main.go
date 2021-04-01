package main

import (
	"fmt"
	"os"

	"github.com/concourse/baggageclaim/baggageclaimcmd"
)

func main() {
	err := baggageclaimcmd.BaggageclaimCommand.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
