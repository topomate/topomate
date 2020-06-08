package config

import (
	"fmt"
)

type LinkModuleInterface interface {
	ApplyConfig()
}

type Link struct {
	Source string
	Dest   string
	Speed  int
}

type LinkModuleSpecs interface {
	ParseSpecs()
}

type LinkModuleManual struct {
	Items []struct {
		From Router
		To   Router
	}
}

type LinkModule struct {
	Kind  string              `yaml:"kind"`
	Specs []map[string]string `yaml:"specs"`
}

func (i *LinkModule) ApplyConfig() {
	fmt.Println("Config applied.")
}

func (l *LinkModuleManual) ParseSpecs() {
	fmt.Println("Oui.")
}
