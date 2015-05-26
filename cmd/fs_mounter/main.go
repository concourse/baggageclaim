package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/concourse/baggageclaim/fs"
	"github.com/pivotal-golang/lager"
)

var diskImage = flag.String(
	"diskImage",
	"",
	"file where disk image will be stored",
)

var mountPath = flag.String(
	"mountPath",
	"",
	"where to mount the filesystem",
)

var sizeInMegabytes = flag.Uint(
	"sizeInMegabytes",
	0,
	"size of the filesystem in megabytes",
)

var remove = flag.Bool(
	"remove",
	false,
	"should we remove the filesystem",
)

func main() {
	flag.Parse()

	if *diskImage == "" {
		fmt.Fprintln(os.Stderr, "-diskImage must be specified")
		os.Exit(1)
	}

	if *mountPath == "" {
		fmt.Fprintln(os.Stderr, "-mountPath must be specified")
		os.Exit(1)
	}

	logger := lager.NewLogger("baggageclaim")
	sink := lager.NewWriterSink(os.Stdout, lager.DEBUG)
	logger.RegisterSink(sink)

	filesystem := fs.New(logger, *diskImage, *mountPath)

	var err error
	if !*remove {
		if *sizeInMegabytes == 0 {
			fmt.Fprintln(os.Stderr, "-sizeInMegabytes must be specified")
			os.Exit(1)
		}

		err = filesystem.Create(uint64(*sizeInMegabytes) * 1024 * 1024)
	} else {
		err = filesystem.Delete()
	}

	if err != nil {
		log.Fatalln("failed to filesystem: ", err)
	}
}
