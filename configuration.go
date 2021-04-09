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
	InfluxDataBase struct {
		Host string `yaml:"host"`
		User string `yaml:"user"`
		Pass string `yaml:"pass"`
	} `yaml:"influxdatabase"`
	BitProphetServiceClient struct {
		DefaultSubList []string `yaml:"defaultsubscribelist"`
		WSHost         string   `yaml:"wshost"`
	} `yaml:"bitprophetserviceclient"`
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
		logger.Printf("Host: %s \tPath: %s", s.Web.Host, s.Web.Path)
		logger.Printf("Cert: %s \tKey: %s", s.Web.CertFile, s.Web.KeyFile)
		logger.Printf("WSHost: %s", s.BitProphetServiceClient.WSHost)
		logger.Printf("InfluxHost: %s", s.InfluxDataBase.Host)
	}
	return nil
}
