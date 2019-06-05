package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/client"
	"github.com/jessevdk/go-flags"
)

var ErrVolumeDoesNotExist = errors.New("volume does not exist")

type PluginCommand struct {
	CreateCommand    CreateCommand    `command:"create"`
	DeleteCommand    DeleteCommand    `command:"delete"`
	ListCommand      ListCommand      `command:"list"`
	InitStoreCommand InitStoreCommand `command:"init-store"`

	BaggageclaimUrl string `long:"baggageclaimURL" required:"true" description:"Address to Baggageclaim Server"`
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
}

func (cc *CreateCommand) Execute(args []string) error {
	client := client.New(Plugin.BaggageclaimUrl, defaultRoundTripper)

	rootfsURL, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	dir, _ := path.Split(rootfsURL.Path)
	handle := path.Base(dir)

	logger.Debug("create-volume", lager.Data{"path": rootfsURL.Path, "handle": handle})

	parentVolume, found, err := client.LookupVolume(logger, handle)

	if !found {
		logger.Error("could not find parent volume", err)
		return ErrVolumeDoesNotExist
	}

	if err != nil {
		logger.Error("failed to find parent volume", err)
		return err
	}

	parentPrivileged, err := parentVolume.GetPrivileged()
	if err != nil {
		logger.Error("could not get privilege of parent volume", err)
		return err
	}

	volume, err := client.CreateVolume(
		logger,
		cc.Handle,
		baggageclaim.VolumeSpec{
			Strategy: baggageclaim.COWStrategy{
				Parent: NewPluginVolume(rootfsURL.Path, handle),
			},
			Privileged: parentPrivileged,
		},
	)
	if err != nil {
		logger.Error("could not create COW volume", err, lager.Data{"args": args})
		return err
	}

	runtimeSpec := &specs.Spec{
		Root: &specs.Root{
			Path:     volume.Path(),
			Readonly: false,
		},
	}

	logger.Debug("created-cow-volume", lager.Data{"path": volume.Path()})

	b, _ := json.Marshal(runtimeSpec)
	fmt.Println(string(b))
	return nil
}

func (dc *DeleteCommand) Execute(args []string) error {
	client := client.New(Plugin.BaggageclaimUrl, defaultRoundTripper)

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
	client := client.New(Plugin.BaggageclaimUrl, defaultRoundTripper)
	volumes, err := client.ListVolumes(logger, nil)
	if err != nil {
		return err
	}

	logger.Debug("list-volumes", lager.Data{"volumes": volumes})
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

var Plugin PluginCommand
var logger lager.Logger

func main() {
	logger = lager.NewLogger("baggageclaim_plugin")
	sink := lager.NewWriterSink(os.Stderr, lager.DEBUG)
	logger.RegisterSink(sink)

	parser := flags.NewParser(&Plugin, flags.HelpFlag|flags.PrintErrors|flags.IgnoreUnknown)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}
}
