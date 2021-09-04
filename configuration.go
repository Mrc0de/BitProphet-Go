package main

import (
	"errors"
	"fmt"
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
	BPData struct {
		Host     string `yaml:"host"`
		User     string `yaml:"user"`
		Pass     string `yaml:"pass"`
		Database string `yaml:"database"`
		DSN      string
	} `yaml:"bpdata"`
	BitProphetServiceClient struct { // Connects to Coinbase and Influx
		DefaultSubscriptions []string `yaml:"defaultsubscriptions"`
		WSHost               string   `yaml:"wshost"`
	} `yaml:"bitprophetserviceclient"`
	BPInternalAccount struct { // Internal Demo account
		Enabled        bool
		AccessKey      string   `yaml:"accesskey"`
		Secret         string   `yaml:"secret"`
		PassPhrase     string   `yaml:"passphrase"`
		DefaultCoins   []string `yaml:"defaultcoins"`
		NativeCurrency string   `yaml:"nativecurrency"`
	} `yaml:"bpinternalaccount"`
	CBVersion   string `yaml:"cbversion"`
	BotDefaults struct {
		MinCryptoBuy     float64  `yaml:"min_crypto_buy"`
		MinPercentProfit float64  `yaml:"min_percent_profit"`
		MaxUSDBuy        float64  `yaml:"max_usd_buy"`
		FeePercent       float64  `yaml:"fee_percent"` // 0.50% of native currency amount
		Markets          []string `yaml:"markets"`
		QuoteIncrements  []string `yaml:"quote_increments"`
		SkipTicksOnBuy   int      `yaml:"skip_ticks_on_buy"`
		SkipTicksOnSell  int      `yaml:"skip_ticks_on_sell"`
	} `yaml:"bot_defaults"`
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
	if len(s.BPInternalAccount.AccessKey) > 0 && len(s.BPInternalAccount.Secret) > 0 && len(s.BPInternalAccount.PassPhrase) > 0 {
		s.BPInternalAccount.Enabled = true
	}
	logger.Printf("(\\.....\\.....,/)")
	logger.Printf(".\\(....|\\....)/")
	logger.Printf(".//\\...| \\../\\\\")
	logger.Printf("(/./\\_#oo#_/\\.\\)")
	logger.Printf(".\\/\\..####../\\/")
	logger.Printf("......`##'......")
	logger.Printf("[!] bitProphet [!]")
	if Debug {
		logger.Printf("Host: %s ", s.Web.Host)
		logger.Printf("Path: %s", s.Web.Path)
		logger.Printf("Cert: %s", s.Web.CertFile)
		logger.Printf("Key: %s", s.Web.KeyFile)
		logger.Printf("WSHost: %s", s.BitProphetServiceClient.WSHost)
		logger.Printf("InfluxHost: %s", s.InfluxDatabase.Host)
		logger.Printf("Internal Account Enabled: %t", s.BPInternalAccount.Enabled)
		logger.Printf("BitProphet DBHost: %s", s.BPData.Host)
	}
	if len(s.BPData.Host) < 1 {
		return errors.New("BPDATA Host is required!")
	}
	s.BPData.DSN = fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", s.BPData.User, s.BPData.Pass, s.BPData.Host, s.BPData.Database)
	return nil
}
