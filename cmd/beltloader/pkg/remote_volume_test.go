package pkg_test

import (
	"testing"

	"github.com/concourse/baggageclaim/cmd/beltloader/pkg"
	"github.com/stretchr/testify/assert"
)

func TestNewRemoteVolume(t *testing.T) {
	for _, tc := range []struct {
		desc       string
		input      string
		shouldFail bool
		expected   pkg.RemoteVolume
	}{
		{
			desc:       "empty",
			input:      "",
			shouldFail: true,
		},
		{
			desc:       "empty parts",
			input:      ",",
			shouldFail: true,
		},
		{
			desc:       "non-empty, but not map-a-like parts",
			input:      "a,a",
			shouldFail: true,
		},
		{
			desc:  "success",
			input: "src=a,dst=b",
			expected: pkg.RemoteVolume{
				Source:      "a",
				Destination: "b",
			},
		},
		{
			desc:  "success in opposite order",
			input: "dst=b,src=a",
			expected: pkg.RemoteVolume{
				Source:      "a",
				Destination: "b",
			},
		},
		{
			desc:  "empty src",
			input: "src=,dst=b",
			expected: pkg.RemoteVolume{
				Source:      "",
				Destination: "b",
			},
		},
		{
			desc:  "empty src and dst",
			input: "src=,dst=",
			expected: pkg.RemoteVolume{
				Source:      "",
				Destination: "",
			},
		},
		{
			desc:       "w/ extra field, fails",
			input:      "src=a,dst=b,foo=bar",
			shouldFail: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			actual, err := pkg.NewRemoteVolume(tc.input)
			if tc.shouldFail {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
