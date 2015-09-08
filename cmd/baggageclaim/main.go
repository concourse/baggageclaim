package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/bomberman"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
)

var listenAddress = flag.String(
	"listenAddress",
	"0.0.0.0",
	"address to listen on",
)

var listenPort = flag.Int(
	"listenPort",
	7788,
	"port for the server to listen on",
)

var volumeDir = flag.String(
	"volumeDir",
	"",
	"directory where volumes and metadata will be stored",
)

var driverType = flag.String(
	"driverType",
	"",
	"the backend driver to use for filesystems",
)

var defaultTTL = flag.Uint(
	"defaultTTL",
	86400, // 1 day in seconds
	"default volume ttl",
)

func main() {
	flag.Parse()
	if *volumeDir == "" {
		fmt.Fprintln(os.Stderr, "-volumeDir must be specified")
		os.Exit(1)
	}

	logger := lager.NewLogger("baggageclaim")
	sink := lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, lager.DEBUG), lager.INFO)
	logger.RegisterSink(sink)

	listenAddr := fmt.Sprintf("%s:%d", *listenAddress, *listenPort)

	var volumeDriver volume.Driver

	if *driverType == "btrfs" {
		volumeDriver = driver.NewBtrFSDriver(logger.Session("driver"))
	} else {
		volumeDriver = &driver.NaiveDriver{}
	}

	ttl := volume.TTL(*defaultTTL)
	locker := volume.NewLockManager()
	volumeRepo := volume.NewRepository(
		logger.Session("repository"),
		volumeDriver,
		locker,
		*volumeDir,
		ttl,
	)

	daBombRepo := bomberman.NewBombermanRepository(volumeRepo, logger.Session("bomberman"))

	apiHandler, err := api.NewHandler(
		logger.Session("api"),
		daBombRepo,
	)
	if err != nil {
		logger.Fatal("failed-to-create-handler", err)
	}

	memberGrouper := []grouper.Member{
		{"api", http_server.New(listenAddr, apiHandler)},
	}

	group := grouper.NewParallel(os.Interrupt, memberGrouper)
	running := ifrit.Invoke(sigmon.New(group))

	logger.Info("listening", lager.Data{
		"addr": listenAddr,
	})

	err = <-running.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}
}
