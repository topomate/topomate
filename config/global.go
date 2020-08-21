package config

var VFlag bool
var ASOnly []int
var ConfigDir string

var DefaultBGPSettings GlobalBGPConfig

const (
	DockerRouterImage = "topomate/router"
	DockerRSImage     = "topomate/route-server"
	DockerRTRImage    = "topomate/rtr"
)
