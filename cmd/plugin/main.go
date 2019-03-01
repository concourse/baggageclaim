package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/client"
	"github.com/jessevdk/go-flags"
)

type PluginCommand struct {
	CreateCommand CreateCommand `command:"create"`
	DeleteCommand DeleteCommand `command:"delete"`
	ListCommand   ListCommand   `command:"list"`
}

type CreateCommand struct {
	Handle string `long:"handle" required:"true" description:"Handle to Create"`
	ApiUrl string `long:"apiURL" required:"true" description:"Address to Baggageclaim Server"`
}

type DeleteCommand struct {
	Handle string `long:"handle" required:"true" description:"Handle to Delete"`
	ApiUrl string `long:"apiURL" required:"true" description:"Address to Baggageclaim Server"`
}

type ListCommand struct {
	ApiUrl string `long:"apiURL" required:"true" description:"Address to Baggageclaim Server"`
}

func (cc *CreateCommand) Execute(args []string) error {
	logger := lager.NewLogger("baggageclaim_client")
	sink := lager.NewWriterSink(os.Stdout, lager.DEBUG)
	logger.RegisterSink(sink)

	client := client.New(cc.ApiUrl, defaultRoundTripper)

	vol, err := client.CreateVolume(logger, cc.Handle, baggageclaim.VolumeSpec{})
	if err != nil {
		return err
	}

	fmt.Println("whats in here", vol)
	return nil
}

func (dc *DeleteCommand) Execute(args []string) error {
	logger := lager.NewLogger("baggageclaim_client")
	sink := lager.NewWriterSink(os.Stdout, lager.DEBUG)
	logger.RegisterSink(sink)

	client := client.New(dc.ApiUrl, defaultRoundTripper)
	handles := []string{dc.Handle}

	err := client.DestroyVolumes(logger, handles)
	if err != nil {
		return err
	}
	return nil
}

func (lc *ListCommand) Execute(args []string) error {
	logger := lager.NewLogger("baggageclaim_client")
	sink := lager.NewWriterSink(os.Stdout, lager.DEBUG)
	logger.RegisterSink(sink)

	client := client.New(lc.ApiUrl, defaultRoundTripper)
	volumes, err := client.ListVolumes(logger, nil)
	if err != nil {
		return err
	}

	logger.Debug("got these volumes", lager.Data{"volumes": volumes})
	return nil
}

var defaultRoundTripper http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSHandshakeTimeout: 10 * time.Second,
}

func main() {
	cmd := &PluginCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}
}
