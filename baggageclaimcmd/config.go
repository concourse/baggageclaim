package baggageclaimcmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/uidgid"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/flag"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

type BaggageclaimConfig struct {
	Logger flag.Lager `yaml:",inline"`

	BindIP   net.IP `yaml:"bind_ip,omitempty"`
	BindPort uint16 `yaml:"bind_port,omitempty"`

	Debug DebugConfig `yaml:"debug,omitempty"`

	P2p P2pConfig `yaml:"p2p,omitempty"`

	VolumesDir flag.Dir `yaml:"volumes,omitempty"`

	Driver string `yaml:"driver,omitempty" validate:"baggageclaim_driver"`

	BtrfsBin string `yaml:"btrfs_bin,omitempty"`
	MkfsBin  string `yaml:"mkfs_bin,omitempty"`

	OverlaysDir string `yaml:"overlays_dir,omitempty"`

	DisableUserNamespaces bool `yaml:"disable_user_namespaces,omitempty"`
}

type DebugConfig struct {
	BindIP   net.IP `yaml:"bind_ip,omitempty"`
	BindPort uint16 `yaml:"bind_port,omitempty"`
}

type P2pConfig struct {
	InterfaceNamePattern string `yaml:"interface_name_pattern,omitempty"`
	InterfaceFamily      int    `yaml:"interface_family,omitempty" validate:"oneof=4 6"`
}

var CmdDefaults = BaggageclaimConfig{
	Logger: flag.Lager{
		LogLevel: "info",
	},

	BindIP:   net.IPv4(127, 0, 0, 1),
	BindPort: 7788,

	Debug: DebugConfig{
		BindIP:   net.IPv4(127, 0, 0, 1),
		BindPort: 7787,
	},

	P2p: P2pConfig{
		InterfaceNamePattern: "eth0",
		InterfaceFamily:      4,
	},

	Driver: "detect",

	BtrfsBin: "btrfs",
	MkfsBin:  "mkfs.btrfs",
}

func (cmd *BaggageclaimConfig) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *BaggageclaimConfig) Runner(args []string) (ifrit.Runner, error) {
	err := cmd.validate()
	if err != nil {
		return nil, err
	}

	logger, _ := cmd.constructLogger()

	listenAddr := fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)

	var privilegedNamespacer, unprivilegedNamespacer uidgid.Namespacer

	if !cmd.DisableUserNamespaces && uidgid.Supported() {
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

	err = driver.Recover(filesystem)
	if err != nil {
		logger.Error("failed-to-recover-volume-driver", err)
		return nil, err
	}

	volumeRepo := volume.NewRepository(
		filesystem,
		locker,
		privilegedNamespacer,
		unprivilegedNamespacer,
	)

	re, err := regexp.Compile(cmd.P2p.InterfaceNamePattern)
	if err != nil {
		logger.Error("failed-to-compile-p2p-interface-name-pattern", err)
		return nil, err
	}
	apiHandler, err := api.NewHandler(
		logger.Session("api"),
		volume.NewStrategerizer(),
		volumeRepo,
		re,
		cmd.P2p.InterfaceFamily,
		cmd.BindPort,
	)
	if err != nil {
		logger.Fatal("failed-to-create-handler", err)
	}

	members := []grouper.Member{
		{Name: "api", Runner: http_server.New(listenAddr, apiHandler)},
		{Name: "debug-server", Runner: http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},
	}

	return onReady(grouper.NewParallel(os.Interrupt, members), func() {
		logger.Info("listening", lager.Data{
			"addr": listenAddr,
		})
	}), nil
}

func (cmd *BaggageclaimConfig) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("baggageclaim")

	return logger, reconfigurableSink
}

func (cmd *BaggageclaimConfig) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.Debug.BindIP, cmd.Debug.BindPort)
}

func (cmd *BaggageclaimConfig) validate() error {
	if cmd.VolumesDir == "" {
		return errors.New("volumes_dir is required")
	}

	return nil
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
