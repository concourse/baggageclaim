package main

import (
	"encoding/json"
	"fmt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/client"
	"github.com/jessevdk/go-flags"
)

type PluginCommand struct {
	CreateCommand    CreateCommand    `command:"create"`
	DeleteCommand    DeleteCommand    `command:"delete"`
	ListCommand      ListCommand      `command:"list"`
	InitStoreCommand InitStoreCommand `command:"init-store"`

	foo string `long:"store-size-bytes" required:"true" description:"Address to Baggageclaim Server"`
	//ImagePluginExtraArg string `long:"image-plugin--arg" required:"true" description:"Address to Baggageclaim Server"`
}

type CreateCommand struct {
	Path   string `required:"true" positional-args:"yes" description:"Path to rootfs"`
	Handle string `required:"true" positional-args:"yes" description:"Handle to Create"`
}

type DeleteCommand struct {
	Handle string `required:"true" positional-args:"yes" description:"Handle to Delete"`
}

type InitStoreCommand struct {
	StoreSizeBytes string `long:"store-size-bytes" required:"true" description:"Address to Baggageclaim Server"`
}

type ListCommand struct {
	ApiUrl string `long:"apiURL" required:"true" description:"Address to Baggageclaim Server"`
}

func (cc *CreateCommand) Execute(args []string) error {
	logger := lager.NewLogger("baggageclaim_plugin")
	sink := lager.NewWriterSink(os.Stderr, lager.DEBUG)
	logger.RegisterSink(sink)

	client := client.New("http://localhost:7788", defaultRoundTripper)

	rootfsURL, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	dir, _ := path.Split(rootfsURL.Path)
	handle := path.Base(dir)
	logger.Debug("creating volume", lager.Data{"path":rootfsURL.Path, "handle":handle})
	vol, err := client.CreateVolume(
		logger,
		cc.Handle,
		baggageclaim.VolumeSpec{
			Strategy: baggageclaim.COWStrategy{
				Parent: NewCantTellYouNothingVolume(rootfsURL.Path, handle),
			},
			Privileged: false, ///TODO: Set this to a sane value
		},
	)
	if err != nil {
		logger.Error("could not create COW volume", err, lager.Data{"args":args})
		return err
	}

	runtimeSpec := &specs.Spec{
		Root: &specs.Root{
			Path: vol.Path(),
			Readonly: false,
		},
	}

	logger.Debug("created-cow-volume", lager.Data{"path":vol.Path()})

	b, _ := json.Marshal(runtimeSpec)
	fmt.Println(string(b))
	return nil
}

func (dc *DeleteCommand) Execute(args []string) error {
	logger := lager.NewLogger("baggageclaim_client")
	sink := lager.NewWriterSink(os.Stderr, lager.DEBUG)
	logger.RegisterSink(sink)

	client := client.New("http://localhost:7788", defaultRoundTripper)

	err := client.DestroyVolume(logger, dc.Handle)
	if err != nil {
		return err
	}
	return nil
}

func (lc *InitStoreCommand) Execute(args []string) error {
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

	parser := flags.NewParser(cmd, flags.HelpFlag|flags.PrintErrors|flags.IgnoreUnknown)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}
}
