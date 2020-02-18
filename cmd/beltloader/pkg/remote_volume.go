package pkg

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/concourse/go-archive/tgzfs"
)

type RemoteVolume struct {
	Source      string
	Destination string
}

func (v RemoteVolume) Pull(ctx context.Context) (err error) {
	request, err := http.NewRequest("PUT", v.Source, nil)
	if err != nil {
		err = fmt.Errorf("new request: %w", err)
		return
	}

	request.Header.Set("Accept-Encoding", "gzip")
	request = request.WithContext(ctx)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		err = fmt.Errorf("do request: %w")
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(response.Body)

		// TODO look at `body`
		//
		err = fmt.Errorf("not ok: %s - %s", response.Status, string(b))
		return
	}

	err = tgzfs.Extract(response.Body, v.Destination)
	if err != nil {
		err = fmt.Errorf("extracting: %w", err)
		return
	}

	return
}

func NewRemoteVolume(raw string) (v RemoteVolume, err error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		err = fmt.Errorf("expected 2 parts from input uri")
		return
	}

	m := make(map[string]string, len(parts))
	for _, part := range parts {
		subParts := strings.Split(part, "=")
		if len(subParts) != 2 {
			err = fmt.Errorf("expected 2 parts from subpart")
			return
		}

		//key            value
		m[subParts[0]] = subParts[1]
	}

	v.Source, v.Destination = m["src"], m["dst"]
	return
}
