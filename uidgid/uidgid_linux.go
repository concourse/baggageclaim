package uidgid

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func (t *translator) getuidgid(info os.FileInfo) (int, int, error) {
	return int(info.Sys().(*syscall.Stat_t).Uid), int(info.Sys().(*syscall.Stat_t).Gid), nil
}

func (t *translator) setuidgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
		Credential: &syscall.Credential{
			Uid: 0,
			Gid: 0,
		},
		GidMappingsEnableSetgroups: true,
	}

	t.mappings.Apply(cmd)
}

type mappings struct {
	uids []syscall.SysProcIDMap
	gids []syscall.SysProcIDMap
}

func newMappings(maxID int) StringMapper {
	return mappings{
		uids: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: maxID, Size: 1},
			{ContainerID: 1, HostID: 1, Size: maxID - 1},
		},
		gids: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: maxID, Size: 1},
			{ContainerID: 1, HostID: 1, Size: maxID - 1},
		},
	}
}

func (m mappings) Apply(cmd *exec.Cmd) {
	cmd.SysProcAttr.UidMappings = m.uids
	cmd.SysProcAttr.GidMappings = m.gids
}

func findMapping(idMap []syscall.SysProcIDMap, fromID int) int {
	for _, id := range idMap {
		if delta := fromID - id.ContainerID; delta < id.Size {
			return id.HostID + delta
		}
	}

	return fromID
}

func (m mappings) Map(fromUid int, fromGid int) (int, int) {
	return findMapping(m.uids, fromUid), findMapping(m.gids, fromGid)
}

func stringifyParts(idMap []syscall.SysProcIDMap) []string {
	if len(idMap) == 0 {
		return []string{"empty"}
	}

	var parts []string
	for _, entry := range idMap {
		parts = append(parts, fmt.Sprintf("%d-%d-%d", entry.ContainerID, entry.HostID, entry.Size))
	}
	return parts
}

func (m mappings) String() string {
	uids := strings.Join(stringifyParts(m.uids), ",")
	gids := strings.Join(stringifyParts(m.gids), ",")
	return fmt.Sprintf("%s+%s", uids, gids)
}
