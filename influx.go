package main

import (
	"encoding/json"
	"fmt"
	client "github.com/influxdata/influxdb1-client"
	"net/url"
	"strconv"
	"time"
)

type influx struct {
	Client *client.Client
}

type PriceRange struct {
	MinPrice float64 `json:"min_price"`
	MaxPrice float64 `json:"max_price"`
}

func (i *influx) Connect() error {
	host, err := url.Parse(fmt.Sprintf("https://%s:%d", Config.InfluxDatabase.Host, 8086))
	if err != nil {
		return err
	}
	con, err := client.NewClient(client.Config{
		URL:       *host,
		Username:  Config.InfluxDatabase.User,
		Password:  Config.InfluxDatabase.Pass,
		UserAgent: "BitProphet-Go",
		Timeout:   2 * time.Second,
	})
	if err != nil {
		return err
	}
	i.Client = con
	return nil
}

func (i *influx) WriteCoinbaseTicker(ticker CoinbaseMessage) error {
	a, err := strconv.ParseFloat(ticker.BestAsk, 32)
	if err != nil {
		return err
	}
	b, err := strconv.ParseFloat(ticker.BestBid, 32)
	if err != nil {
		return err
	}
	p, err := strconv.ParseFloat(ticker.Price, 32)
	if err != nil {
		return err
	}

	pt := client.Point{
		Measurement: "tickers",
		Tags: map[string]string{
			"market": ticker.ProductID,
		},
		Fields: map[string]interface{}{
			"ask":   a,
			"bid":   b,
			"price": p,
		},
		Time:      time.Now(),
		Precision: "s",
	}
	bp := client.BatchPoints{
		Points:    []client.Point{pt},
		Database:  "coinbasePriceHistory",
		Time:      time.Now(),
		Tags:      pt.Tags,
		Precision: "s",
	}
	_, err = i.Client.Write(bp)
	if err != nil {
		return err
	}
	return nil
}

func (i *influx) GetMinMaxPrices(market string, maxHours int) (PriceRange, error) {
	//default maxHours = 4
	if maxHours == 0 {
		maxHours = 4
	}
	pr := PriceRange{}
	q := client.Query{
		Command: fmt.Sprintf("SELECT min(price) as minPrice, max(price) as maxPrice FROM tickers where market='%s' and time > now()-%dh;",
			market, maxHours),
		Database: "coinbasePriceHistory",
	}
	resp, err := i.Client.Query(q)
	// basic error
	if err != nil {
		logger.Printf("[GetMinMaxPrices] Influx Query Failure: %s", err)
		return pr, err
	}
	if resp.Error() != nil {
		logger.Printf("[GetMinMaxPrices] Influx Query Response Failure: %s", err)
		return pr, err
	}
	for _, topval := range resp.Results {
		for _, sval := range topval.Series {
			minp, err := strconv.ParseFloat(string(sval.Values[0][1].(json.Number)), 32)
			if err != nil {
				logger.Printf("[GetMinMaxPrices] ParseFloat Error: %s", err)
				return pr, err
			}
			maxp, err := strconv.ParseFloat(string(sval.Values[0][2].(json.Number)), 32)
			if err != nil {
				logger.Printf("[GetMinMaxPrices] ParseFloat Error: %s", err)
				return pr, err
			}
			logger.Printf("[GetMinMaxPrices] MinPrice: $%.2f \t MaxPrice: $%.2f", minp, maxp)
			pr.MinPrice = minp
			pr.MaxPrice = maxp
		}
	}
	return pr, err
}
