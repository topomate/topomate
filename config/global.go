package config

const (
	NetOVS = iota
	NetODP = iota
)

var VFlag bool
var ASOnly []int
var ConfigDir string

var DefaultBGPSettings GlobalBGPConfig
var NetBackend int

const (
	DockerRouterImage = "topomate/router"
	DockerRSImage     = "topomate/route-server"
)

const (
	DefaultDir        = "~/topomate"
	DefaultProjectDir = DefaultDir + "/projects"
	DefaultConfigDir  = DefaultDir + "/generated"
)
