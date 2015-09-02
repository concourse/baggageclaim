package baggageclaim

import "github.com/tedsuo/rata"

const (
	GetVolumes   = "GetVolumes"
	CreateVolume = "CreateVolume"

	SetProperty = "SetProperty"
)

var Routes = rata.Routes{
	{Path: "/volumes", Method: "GET", Name: GetVolumes},
	{Path: "/volumes", Method: "POST", Name: CreateVolume},

	{Path: "/volumes/:handle/properties/:property", Method: "PUT", Name: SetProperty},
}
