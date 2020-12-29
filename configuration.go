package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Configuration struct {
	Web struct {
		Listen   string `yaml:"listen"` // interfaces to listen on
		Host     string `yaml:"host"`   // servers hostname for public web clients (ie: geekprojex.com )
		Path     string `yaml:"path"`
		CertFile string `yaml:"certfile"`
		KeyFile  string `yaml:"keyfile"`
	} `yaml:"web"`
}

func (s *Configuration) load(confFile string) error {
	f, err := ioutil.ReadFile(confFile)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(f, &s)
	if err != nil {
		return err
	}
	if Debug {
		logger.Printf("Host: %s \tPath: %s \tCert: %s \tKey: %s", Config.Web.Host, Config.Web.Path,
			Config.Web.CertFile, Config.Web.KeyFile)
	}
	return nil
}
