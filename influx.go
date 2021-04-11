package main

import (
	"fmt"
	client "github.com/influxdata/influxdb1-client"
	"net/url"
	"time"
)

type influx struct {
	Client *client.Client
}

func (i *influx) Connect() error {
	host, err := url.Parse(fmt.Sprintf("https://%s:%d", Config.InfluxDatabase.Host, 8086))
	if err != nil {
		return err
	}
	con, err := client.NewClient(client.Config{
		URL:        *host,
		UnixSocket: "",
		Username:   Config.InfluxDatabase.User,
		Password:   Config.InfluxDatabase.Pass,
		UserAgent:  "BitProphet-Go",
		Timeout:    3 * time.Second,
	})
	if err != nil {
		return err
	}
	i.Client = con
	return nil
}

func (i *influx) WriteCoinbaseTicker(ticker CoinbaseMessage) error {
	pt := client.Point{
		Measurement: "tickers",
		Tags: map[string]string{
			"market": ticker.ProductID,
		},
		Fields: map[string]interface{}{
			"ask":   ticker.BestAsk,
			"bid":   ticker.BestBid,
			"price": ticker.Price,
		},
		Time:      time.Now(),
		Precision: "s",
	}
	bp := client.BatchPoints{
		Points:    []client.Point{pt},
		Database:  "coinbasePriceHistory",
		Time:      time.Now(),
		Precision: "s",
	}
	_, err := i.Client.Write(bp)
	if err != nil {
		return err
	}
	return nil
}
