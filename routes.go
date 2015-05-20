package mattermaster

import "github.com/tedsuo/rata"

const (
	CreateVolume = "CreateVolume"
)

var Routes = rata.Routes{
	{Path: "/volumes", Method: "POST", Name: CreateVolume},
}
