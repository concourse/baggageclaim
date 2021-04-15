package baggageclaimcmd

import (
	"github.com/spf13/cobra"
)

func InitializeBaggageclaimFlags(c *cobra.Command, flags *BaggageclaimConfig, prefix string) {
	c.Flags().IPVar(&flags.BindIP, prefix+"baggageclaim-bind-ip", CmdDefaults.BindIP, "IP address on which to listen for API traffic.")
	c.Flags().Uint16Var(&flags.BindPort, prefix+"baggageclaim-bind-port", CmdDefaults.BindPort, "Port on which to listen for API traffic.")

	c.Flags().IPVar(&flags.Debug.BindIP, prefix+"baggageclaim-debug-bind-ip", CmdDefaults.Debug.BindIP, "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Debug.BindPort, prefix+"baggageclaim-debug-bind-port", CmdDefaults.Debug.BindPort, "Port on which to listen for the pprof debugger endpoints.")

	c.Flags().StringVar(&flags.P2p.InterfaceNamePattern, prefix+"baggageclaim-p2p-interface-name-pattern", CmdDefaults.P2p.InterfaceNamePattern, "Regular expression to match a network interface for p2p streaming")
	c.Flags().IntVar(&flags.P2p.InterfaceFamily, prefix+"baggageclaim-p2p-interface-family", CmdDefaults.P2p.InterfaceFamily, "4 for IPv4 and 6 for IPv6")

	c.Flags().Var(&flags.VolumesDir, prefix+"baggageclaim-volumes", "Directory in which to place volume data.")

	c.Flags().StringVar(&flags.Driver, prefix+"baggageclaim-driver", CmdDefaults.Driver, "Driver to use for managing volumes.")

	c.Flags().StringVar(&flags.BtrfsBin, prefix+"baggageclaim-btrfs-bin", CmdDefaults.BtrfsBin, "Path to btrfs binary")
	c.Flags().StringVar(&flags.MkfsBin, prefix+"baggageclaim-mkfs-bin", CmdDefaults.MkfsBin, "Path to mkfs.btrfs binary")

	c.Flags().StringVar(&flags.OverlaysDir, prefix+"baggageclaim-overlays-dir", "", "Path to directory in which to store overlay data")

	c.Flags().BoolVar(&flags.DisableUserNamespaces, prefix+"baggageclaim-disable-user-namespaces", false, "Disable remapping of user/group IDs in unprivileged volumes.")
}
