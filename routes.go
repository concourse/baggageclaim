package baggageclaim

import "github.com/tedsuo/rata"

const (
	ListVolumes    = "ListVolumes"
	GetVolume      = "GetVolume"
	GetVolumeStats = "GetVolumeStats"
	CreateVolume   = "CreateVolume"
	DestroyVolume  = "DestroyVolume"

	SetProperty = "SetProperty"
	SetTTL      = "SetTTL"
	StreamIn    = "StreamIn"
	StreamOut   = "StreamOut"
)

var Routes = rata.Routes{
	{Path: "/volumes", Method: "GET", Name: ListVolumes},
	{Path: "/volumes", Method: "POST", Name: CreateVolume},

	{Path: "/volumes/:handle", Method: "GET", Name: GetVolume},
	{Path: "/volumes/:handle/stats", Method: "GET", Name: GetVolumeStats},
	{Path: "/volumes/:handle/properties/:property", Method: "PUT", Name: SetProperty},
	{Path: "/volumes/:handle/ttl", Method: "PUT", Name: SetTTL},
	{Path: "/volumes/:handle/stream-in", Method: "PUT", Name: StreamIn},
	{Path: "/volumes/:handle/stream-out", Method: "PUT", Name: StreamOut},
	{Path: "/volumes/:handle", Method: "DELETE", Name: DestroyVolume},
}
