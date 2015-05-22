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

	apiHandler, err := api.NewHandler(
		logger.Session("api"),
		*volumeDir,
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
