package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/concourse/mattermaster/fs"
)

var diskImage = flag.String(
	"diskImage",
	"",
	"file where disk image will be stored",
)

var loopbackDevice = flag.String(
	"loopbackDevice",
	"",
	"loopback device to use",
)

var mountPath = flag.String(
	"mountPath",
	"",
	"where to mount the filesystem",
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

	if *loopbackDevice == "" {
		fmt.Fprintln(os.Stderr, "-loopbackDevice must be specified")
		os.Exit(1)
	}

	if *mountPath == "" {
		fmt.Fprintln(os.Stderr, "-mountPath must be specified")
		os.Exit(1)
	}

	var err error
	if !*remove {
		err = fs.CreateBtrFSVolume(*diskImage, *loopbackDevice, *mountPath)
	} else {
		err = fs.DeleteBtrFSVolume(*diskImage, *loopbackDevice, *mountPath)
	}

	if err != nil {
		log.Fatalln(err)
	}
}
