package mattermaster

import "github.com/tedsuo/rata"

const (
	CreateVolume = "CreateVolume"
	GetVolumes   = "GetVolumes"
)

var Routes = rata.Routes{
	{Path: "/volumes", Method: "POST", Name: CreateVolume},
	{Path: "/volumes", Method: "GET", Name: GetVolumes},
}
