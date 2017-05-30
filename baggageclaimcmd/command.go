package baggageclaimcmd

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/reaper"
	"github.com/concourse/baggageclaim/uidgid"
	"github.com/concourse/baggageclaim/volume"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/xoebus/zest"
)

type BaggageclaimCommand struct {
	Logger LagerFlag

	BindIP   IPFlag `long:"bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for API traffic."`
	BindPort uint16 `long:"bind-port" default:"7788"      description:"Port on which to listen for API traffic."`

	VolumesDir DirFlag `long:"volumes" required:"true" description:"Directory in which to place volume data."`

	Driver string `long:"driver" default:"detect" choice:"detect" choice:"naive" choice:"btrfs" choice:"overlay" description:"Driver to use for managing volumes."`

	BtrfsBin string `long:"btrfs-bin" default:"btrfs" description:"Path to btrfs binary"`
	MkfsBin  string `long:"mkfs-bin" default:"mkfs.btrfs" description:"Path to mkfs.btrfs binary"`

	OverlaysDir string `long:"overlays-dir" description:"Path to directory in which to store overlay data"`

	ReapInterval time.Duration `long:"reap-interval" default:"10s" description:"Interval on which to reap expired volumes."`

	Metrics struct {
		YellerAPIKey      string `long:"yeller-api-key"     description:"Yeller API key. If specified, all errors logged will be emitted."`
		YellerEnvironment string `long:"yeller-environment" description:"Environment to tag on all Yeller events emitted."`
	} `group:"Metrics & Diagnostics"`
}

func (cmd *BaggageclaimCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *BaggageclaimCommand) Runner(args []string) (ifrit.Runner, error) {
	logger, _ := cmd.constructLogger()

	listenAddr := fmt.Sprintf("%s:%d", cmd.BindIP.IP(), cmd.BindPort)

	var privilegedNamespacer, unprivilegedNamespacer uidgid.Namespacer

	if uidgid.Supported() {
		privilegedNamespacer = &uidgid.UidNamespacer{
			Translator: uidgid.NewTranslator(uidgid.NewPrivilegedMapper()),
			Logger:     logger.Session("uid-namespacer"),
		}

		unprivilegedNamespacer = &uidgid.UidNamespacer{
			Translator: uidgid.NewTranslator(uidgid.NewUnprivilegedMapper()),
			Logger:     logger.Session("uid-namespacer"),
		}
	} else {
		privilegedNamespacer = uidgid.NoopNamespacer{}
		unprivilegedNamespacer = uidgid.NoopNamespacer{}
	}

	locker := volume.NewLockManager()

	driver, err := cmd.driver(logger)
	if err != nil {
		logger.Error("failed-to-set-up-driver", err)
		return nil, err
	}

	filesystem, err := volume.NewFilesystem(driver, cmd.VolumesDir.Path())
	if err != nil {
		logger.Error("failed-to-initialize-filesystem", err)
		return nil, err
	}

	volumeRepo := volume.NewRepository(
		logger.Session("repository"),
		filesystem,
		locker,
		privilegedNamespacer,
		unprivilegedNamespacer,
	)

	apiHandler, err := api.NewHandler(
		logger.Session("api"),
		volume.NewStrategerizer(),
		volumeRepo,
	)
	if err != nil {
		logger.Fatal("failed-to-create-handler", err)
	}

	clock := clock.NewClock()

	morbidReality := reaper.NewReaper(clock, volumeRepo)

	members := []grouper.Member{
		{Name: "api", Runner: http_server.New(listenAddr, apiHandler)},
		{Name: "reaper", Runner: reaper.NewRunner(logger, clock, cmd.ReapInterval, morbidReality.Reap)},
	}

	return onReady(grouper.NewParallel(os.Interrupt, members), func() {
		logger.Info("listening", lager.Data{
			"addr": listenAddr,
		})
	}), nil
}

func (cmd *BaggageclaimCommand) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("baggageclaim")

	if cmd.Metrics.YellerAPIKey != "" {
		yellerSink := zest.NewYellerSink(cmd.Metrics.YellerAPIKey, cmd.Metrics.YellerEnvironment)
		logger.RegisterSink(yellerSink)
	}

	return logger, reconfigurableSink
}

func onReady(runner ifrit.Runner, cb func()) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		process := ifrit.Background(runner)

		subExited := process.Wait()
		subReady := process.Ready()

		for {
			select {
			case <-subReady:
				cb()
				subReady = nil
			case err := <-subExited:
				return err
			case sig := <-signals:
				process.Signal(sig)
			}
		}
	})
}
