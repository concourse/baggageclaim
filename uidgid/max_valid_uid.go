package uidgid

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
)

type IDMap string

const defaultUIDMap IDMap = "/proc/self/uid_map"
const defaultGIDMap IDMap = "/proc/self/gid_map"

func Supported() bool {
	return runtime.GOOS == "linux" &&
		defaultUIDMap.Supported() &&
		defaultGIDMap.Supported()
}

func MustGetMaxValidUID() int {
	return must(defaultUIDMap.MaxValid())
}

func MustGetMaxValidGID() int {
	return must(defaultGIDMap.MaxValid())
}

func (u IDMap) Supported() bool {
	_, err := os.Open(string(u))
	return !os.IsNotExist(err)
}

func (u IDMap) MaxValid() (int, error) {
	f, err := os.Open(string(u))
	if err != nil {
		return 0, err
	}

	m := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var container, host, size int
		if _, err := fmt.Sscanf(scanner.Text(), "%d %d %d", &container, &host, &size); err != nil {
			return 0, ParseError{Line: scanner.Text(), Err: err}
		}

		m = max(m, container+size-1)
	}

	return m, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

type ParseError struct {
	Line string
	Err  error
}

func (p ParseError) Error() string {
	return fmt.Sprintf(`%s while parsing line "%s"`, p.Err, p.Line)
}

func must(a int, err error) int {
	if err != nil {
		panic(err)
	}

	return a
}
