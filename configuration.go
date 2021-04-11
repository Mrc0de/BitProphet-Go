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
	InfluxDatabase struct {
		Host string `yaml:"host"`
		User string `yaml:"user"`
		Pass string `yaml:"pass"`
	} `yaml:"influxdatabase"`
	BitProphetServiceClient struct { // Connects to Coinbase and Influx
		DefaultSubscriptions []string `yaml:"defaultsubscriptions"`
		WSHost               string   `yaml:"wshost"`
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
		logger.Printf("Host: %s ", s.Web.Host)
		logger.Printf("Path: %s", s.Web.Path)
		logger.Printf("Cert: %s", s.Web.CertFile)
		logger.Printf("Key: %s", s.Web.KeyFile)
		logger.Printf("WSHost: %s", s.BitProphetServiceClient.WSHost)
		logger.Printf("InfluxHost: %s", s.InfluxDatabase.Host)
	}
	return nil
}
