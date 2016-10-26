package uidjunk

import "syscall"

func (u UidTranslator) uidMappings() []syscall.SysProcIDMap {
	return []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: u.maxID, Size: 1},
		{ContainerID: 1, HostID: 1, Size: u.maxID - 1},
	}
}

func (u UidTranslator) gidMappings() []syscall.SysProcIDMap {
	return []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: u.maxID, Size: 1},
		{ContainerID: 1, HostID: 1, Size: u.maxID - 1},
	}
}
