package baggageclaimcmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/reaper"
	"github.com/concourse/baggageclaim/uidjunk"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/xoebus/zest"
)

type BaggageclaimCommand struct {
	BindIP   IPFlag `long:"bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for API traffic."`
	BindPort uint16 `long:"bind-port" default:"7788"      description:"Port on which to listen for API traffic."`

	VolumesDir DirFlag `long:"volumes" required:"true" description:"Directory in which to place volume data."`

	Driver   string `long:"driver" default:"naive" choice:"naive" choice:"btrfs" description:"Driver to use for managing volumes."`
	BtrfsBin string `long:"btrfs-bin" default:"btrfs" description:"Path to btrfs binary"`
	MkfsBin  string `long:"mkfs-bin" default:"mkfs.btrfs" description:"Path to mkfs.btrfs binary"`

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
	logger := lager.NewLogger("baggageclaim")
	sink := lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, lager.DEBUG), lager.INFO)
	logger.RegisterSink(sink)

	if cmd.Metrics.YellerAPIKey != "" {
		yellerSink := zest.NewYellerSink(cmd.Metrics.YellerAPIKey, cmd.Metrics.YellerEnvironment)
		logger.RegisterSink(yellerSink)
	}

	listenAddr := fmt.Sprintf("%s:%d", cmd.BindIP.IP(), cmd.BindPort)

	var volumeDriver volume.Driver

	if cmd.Driver == "btrfs" {
		volumeDriver = driver.NewBtrFSDriver(
			logger.Session("driver"),
			string(cmd.VolumesDir),
			cmd.BtrfsBin,
		)
	} else {
		volumeDriver = &driver.NaiveDriver{}
	}

	var namespacer uidjunk.Namespacer

	maxUID, maxUIDErr := uidjunk.DefaultUIDMap.MaxValid()
	maxGID, maxGIDErr := uidjunk.DefaultGIDMap.MaxValid()

	if runtime.GOOS == "linux" && maxUIDErr == nil && maxGIDErr == nil {
		maxId := uidjunk.Min(maxUID, maxGID)

		mappingList := uidjunk.MappingList{
			{
				FromID: 0,
				ToID:   maxId,
				Size:   1,
			},
			{
				FromID: 1,
				ToID:   1,
				Size:   maxId - 1,
			},
		}

		uidTranslator := uidjunk.NewUidTranslator(
			mappingList,
			mappingList,
		)

		namespacer = &uidjunk.UidNamespacer{
			Translator: uidTranslator,
			Logger:     logger.Session("uid-namespacer"),
		}
	} else {
		namespacer = uidjunk.NoopNamespacer{}
	}

	locker := volume.NewLockManager()

	filesystem, err := volume.NewFilesystem(volumeDriver, string(cmd.VolumesDir))
	if err != nil {
		logger.Fatal("failed-to-initialize-filesystem", err)
	}

	volumeRepo := volume.NewRepository(
		logger.Session("repository"),
		filesystem,
		locker,
	)

	strategerizer := volume.NewStrategerizer(namespacer)

	apiHandler, err := api.NewHandler(
		logger.Session("api"),
		strategerizer,
		volumeRepo,
	)
	if err != nil {
		logger.Fatal("failed-to-create-handler", err)
	}

	clock := clock.NewClock()

	morbidReality := reaper.NewReaper(clock, volumeRepo)

	members := []grouper.Member{
		{"api", http_server.New(listenAddr, apiHandler)},
		{"reaper", reaper.NewRunner(logger, clock, cmd.ReapInterval, morbidReality.Reap)},
	}

	return onReady(grouper.NewParallel(os.Interrupt, members), func() {
		logger.Info("listening", lager.Data{
			"addr": listenAddr,
		})
	}), nil
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
