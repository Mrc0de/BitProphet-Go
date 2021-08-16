package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	api "github.com/mrc0de/BitProphet-Go/CoinbaseAPI"
	"strconv"
	"strings"
	"time"
)

type CoinbaseOrderResponse struct {
	Message        string    `json:"message"`
	ID             string    `json:"id"`
	Size           string    `json:"size"`
	ProductId      string    `json:"product_id"`
	Side           string    `json:"side"`
	Stp            string    `json:"stp"`
	Funds          string    `json:"funds"`
	SpecifiedFunds string    `json:"specified_funds"`
	Type           string    `json:"type"`
	PostOnly       bool      `json:"post_only"`
	CreatedAt      time.Time `json:"created_at"`
	DoneAt         time.Time `json:"done_at"`
	DoneReason     string    `json:"done_reason"`
	FillFees       string    `json:"fill_fees"`
	FilledSize     string    `json:"filled_size"`
	ExecutedValue  string    `json:"executed_value"`
	Status         string    `json:"status"`
	Settled        bool      `json:"settled"`
}

type BitProphetLedgerRecord struct {
	ID               uuid.UUID       `json:"id"`
	Market           sql.NullString  `json:"market"`
	Type             sql.NullString  `json:"type"`
	Cost             sql.NullFloat64 `json:"cost"`
	Price            sql.NullFloat64 `json:"price"`
	CoinAmount       sql.NullFloat64 `json:"coin_amount"`
	BuyFee           sql.NullFloat64 `json:"buy_fee"`
	ProjectedSellFee sql.NullFloat64 `json:"projected_sell_fee"`
	Time             sql.NullTime    `json:"time"`
	SellFee          sql.NullFloat64 `json:"sell_fee"`
	SellPrice        sql.NullFloat64 `json:"sell_price"`
	BuyOrderID       sql.NullString  `json:"buy_order_id"`
	SellOrderID      sql.NullString  `json:"sell_order_id"`
	TimeSold         sql.NullTime    `json:"time_sold"`
	Status           sql.NullString  `json:"status"`
	FilledBuy        sql.NullBool    `json:"filled_buy"`
	FilledSell       sql.NullBool    `json:"filled_sell"`
	SoldValue        sql.NullFloat64 `json:"sold_value"`
}

type BitProphetBot struct {
	ServiceChannel     chan *bpServiceEvent
	AutoSuggestChannel chan *bpServiceEvent // text only (debug)
	ParentService      *bpService
}

func CreateBitProphetBot() *BitProphetBot {
	return &BitProphetBot{
		ServiceChannel:     make(chan *bpServiceEvent, 0),
		AutoSuggestChannel: make(chan *bpServiceEvent, 0),
	}
}

func (b *BitProphetBot) Run() {
	b.ServiceChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "BOT",
		EventData: "[BitProphetBot::Run] Started Bot",
	}
	autoSuggestTicker := time.NewTicker(60 * time.Second)
	checkBuyFillsTicker := time.NewTicker(80 * time.Second)
	checkSellFillsTicker := time.NewTicker(100 * time.Second)
	for {
		select {
		case <-autoSuggestTicker.C:
			{
				b.AutoSuggest()
			}
		case <-checkBuyFillsTicker.C:
			{
				b.CheckBuyFills()
			}
		case <-checkSellFillsTicker.C:
			{
				b.CheckSellFills()
			}
		}
	}
}

func (b *BitProphetBot) AutoSuggest() {
	// we need to know the internal account's spendable balance
	// go slowly if possible, dont hammer anything
	// we need the 'current data' FIRST then we analyze the possibles.

	// Documenting old way
	// get wallet balance of primary currency (AVAILABLE BALANCE)
	req := api.NewSecureRequest("list_accounts", Config.CBVersion) // create the req
	req.Credentials.Key = Config.BPInternalAccount.AccessKey       // setup it's creds
	req.Credentials.Passphrase = Config.BPInternalAccount.PassPhrase
	req.Credentials.Secret = Config.BPInternalAccount.Secret
	reply, err := req.Process(logger) // process request
	if err != nil {
		logger.Printf("[AutoSuggest] ERROR: %s", err)
		return
	}
	var accList []api.CoinbaseAccount
	err = json.Unmarshal(reply, &accList)
	if err != nil {
		logger.Printf("[AutoSuggest] ERROR: %s", err)
		return
	}
	var NativeAcc api.CoinbaseAccount
	for _, acc := range accList {
		if acc.Currency == Config.BPInternalAccount.NativeCurrency {
			NativeAcc = acc
		}
	}
	if strings.Contains(NativeAcc.Balance, ".") {
		NativeAcc.Balance = NativeAcc.Balance[:strings.Index(NativeAcc.Balance, ".")+3]
	}
	if strings.Contains(NativeAcc.Available, ".") {
		NativeAcc.Available = NativeAcc.Available[:strings.Index(NativeAcc.Available, ".")+3]
	}
	logger.Printf("[AutoSuggest] Found [%s] Account Total: $%s Available: $%s", NativeAcc.Currency, NativeAcc.Balance, NativeAcc.Available)
	// Now we know our hard limit for spending (our native currency's available balance)
	// Config.BotDefaults.MinCryptoBuy * CoinPrice = Minimum Spend Amount BEFORE FEES
	/////////////////////////////////////////////////////////////////////////////////
	for _, m := range Config.BotDefaults.Markets {
		coinAsk := b.ParentService.CoinPricesNow[m].Ask
		coin := b.ParentService.CoinPricesNow[m].Market
		minPriceBuy := coinAsk * Config.BotDefaults.MinCryptoBuy
		willSpend := 0.0
		logger.Printf("[AutoSuggest] Available [%s] [$%s]", NativeAcc.Currency, NativeAcc.Available)
		logger.Printf("[AutoSuggest] Price [%s] [$%.2f]", m, coinAsk)
		logger.Printf("[AutoSuggest] MinCryptoBuy: (%.2f %s * $%.2f) = MinPriceBuy: $%.2f", Config.BotDefaults.MinCryptoBuy, coin, coinAsk, minPriceBuy)
		availCash, err := strconv.ParseFloat(NativeAcc.Available, 32)
		if err != nil {
			logger.Printf("[AutoSuggest] ERROR: %s", err)
		}
		if availCash < minPriceBuy {
			logger.Printf("[AutoSuggest] Not Enough Available %s, Aborting.", NativeAcc.Currency)
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		logger.Printf("[AutoSuggest] MaxBuy: $%.2f", Config.BotDefaults.MaxUSDBuy)
		if minPriceBuy > Config.BotDefaults.MaxUSDBuy {
			logger.Printf("[AutoSuggest] MinPrice is more than MaxBuy, Aborting.")
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		// We have enough to minimum buy....
		// but instead we will buy UP TO maxBuy OR just min...
		if availCash <= Config.BotDefaults.MaxUSDBuy {
			// we cant buy up to max, just go with min
			if availCash > minPriceBuy {
				willSpend = availCash - (availCash * Config.BotDefaults.FeePercent * 0.01)
			} else {
				willSpend = minPriceBuy
			}
		} else {
			// we can buy up to max, do it
			willSpend = Config.BotDefaults.MaxUSDBuy
		}
		logger.Printf("[AutoSuggest] Will Spend: $%.2f", willSpend)
		// What is the BuyPoint FEE? determine willSpendWithBuyFee...
		buyFee := (Config.BotDefaults.FeePercent * 0.01) * willSpend
		willSpendWithBuyFee := willSpend + buyFee
		logger.Printf("[AutoSuggest] Fee: $%.2f \tTotal: $%.2f", buyFee, willSpendWithBuyFee)
		// how much coin for that much @ current price?
		willBuyCoinAmount := willSpend / coinAsk
		if willSpendWithBuyFee > availCash {
			logger.Printf("[AutoSuggest] Available Balance less than $%.2f, Reverting to $%.2f", willSpendWithBuyFee, minPriceBuy)
			willSpend = minPriceBuy
			buyFee = (Config.BotDefaults.FeePercent * 0.01) * willSpend
			willSpendWithBuyFee = willSpend + buyFee
			logger.Printf("[AutoSuggest] Fee: $%.2f \tTotal: $%.2f", buyFee, willSpendWithBuyFee)
			willBuyCoinAmount = willSpend / coinAsk
			if willSpendWithBuyFee > availCash {
				logger.Printf("[AutoSuggest] Available Balance less than $%.2f, Aborting.", willSpendWithBuyFee)
				logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
				continue
			}
		}
		logger.Printf("[AutoSuggest] Coin Amount: %.8f @ Price: $%.2f For $%.2f ( w/Fee: $%.2f )", willBuyCoinAmount, coinAsk, willSpend, willSpendWithBuyFee)
		// but SHOULD we buy now at current price?
		// determine buffer zone
		// is price within buffer, if so, check sell price probability, if good, purchase and immediately place for sale at sellprice
		// sellprice determined by fees, buy amount and price, as well as, percent profit setting
		logger.Printf("[AutoSuggest] Analyzing Price History for %s", m)
		//  SELECT min(price) as mini ,max(price) as maxi FROM tickers where market='LTC-USD' and time > now()-4h;
		pr, err := b.ParentService.Client.Influx.GetMinMaxPrices(m, 8)
		if err != nil {
			logger.Printf("[AutoSuggest] ERROR: %s, Aborting", err)
			continue
		}
		logger.Printf("[AutoSuggest] Price Range (8h): $%.2f - $%.2f", pr.MinPrice, pr.MaxPrice)
		b.AutoSuggestChannel <- &bpServiceEvent{
			Time:      time.Now(),
			EventType: "AUTO_SUGGEST",
			EventData: fmt.Sprintf("[AutoSuggest] Price Range (8h): $%.2f - $%.2f", pr.MinPrice, pr.MaxPrice),
		}
		// find BUFFERZONE FLOOR (above 5% of gap above floor)
		// find BUFFERZONE CEILING (BELOW 15% of gap below ceiling)
		gap := pr.MaxPrice - pr.MinPrice
		zoneFloor := (gap * 0.05) + pr.MinPrice
		zoneRoof := pr.MaxPrice - (gap * 0.15)
		logger.Printf("[AutoSuggest] Buy Zone: $%.2f - $%.2f", zoneFloor, zoneRoof)
		b.ChatSay(fmt.Sprintf("[AutoSuggest] Buy Zone: $%.2f - $%.2f", zoneFloor, zoneRoof))
		if coinAsk < zoneFloor || coinAsk > zoneRoof {
			logger.Printf("[AutoSuggest] Ask Price $%.2f outside of Buy Zone, ABORTED.", coinAsk)
			b.ChatSay(fmt.Sprintf("[AutoSuggest] Ask Price $%.2f outside of Buy Zone, ABORTED.", coinAsk))
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		logger.Printf("[AutoSuggest] Ask Price $%.2f is within the Buy Zone.", coinAsk)
		// determine sellprice for this buy prospect
		profitNeeded := (Config.BotDefaults.MinPercentProfit * 0.01) * willSpend
		willSellFor := willSpendWithBuyFee + profitNeeded
		sellFee := (Config.BotDefaults.FeePercent * 0.01) * willSellFor
		logger.Printf("[AutoSuggest] [Price $%.2f] [SpendWithFee: $%.2f] [ProfitNeeded: $%.2f] [WillSellFor: $%.2f] [SellFee: $%.2f] "+
			"[Profit: $%.2f] [SellPrice: $%.2f]!",
			coinAsk, willSpendWithBuyFee, profitNeeded, willSellFor, sellFee, willSellFor-sellFee-willSpendWithBuyFee, willSellFor/willBuyCoinAmount)
		if willSellFor-sellFee-willSpendWithBuyFee < 0.01 || willSellFor/willBuyCoinAmount > pr.MaxPrice {
			// less than 1 cent of profit... PASS, no thanks
			logger.Printf("[AutoSuggest] NO PROFIT = NO BUY [PASS on Buying %s]", m[:strings.Index(m, "-")])
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		b.ChatSay(fmt.Sprintf("[AutoSuggest] [Price $%.2f] [SpendWithFee: $%.2f] [ProfitNeeded: $%.2f] [WillSellFor: $%.2f] [SellFee: $%.2f] "+
			"[Profit: $%.2f] [SellPrice: $%.2f]!", coinAsk, willSpendWithBuyFee, profitNeeded, willSellFor, sellFee,
			willSellFor-sellFee-willSpendWithBuyFee, willSellFor/willBuyCoinAmount))
		logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
		// Buy
		breq := api.NewSecureRequest("place_order", Config.CBVersion) // create the req
		breq.Credentials.Key = Config.BPInternalAccount.AccessKey     // setup it's creds
		breq.Credentials.Passphrase = Config.BPInternalAccount.PassPhrase
		breq.Credentials.Secret = Config.BPInternalAccount.Secret
		breq.RequestMethod = "POST"
		var buy struct {
			Size   string `json:"size"`
			Price  string `json:"price"`
			Side   string `json:"side"`
			Market string `json:"product_id"`
		}
		buy.Side = "buy"
		buy.Market = m
		buy.Size = fmt.Sprintf("%.1f", willBuyCoinAmount)
		buy.Price = fmt.Sprintf("%.2f", coinAsk)

		rbody, err := json.Marshal(buy)
		if err != nil {
			logger.Printf("[AutoSuggest] Buy Error: %s", err)
		}
		breq.RequestBody = string(rbody)
		bresp, err := breq.Process(logger) // process request
		if err != nil {
			logger.Printf("[AutoSuggest] Buy Request Error: %s", err)
		}
		jresp := CoinbaseOrderResponse{}
		err = json.Unmarshal(bresp, &jresp)
		if err != nil {
			logger.Printf("[AutoSuggest] Buy Response unmarshall Error: %s", err)
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		if len(jresp.ID) < 1 {
			logger.Printf("[AutoSuggest] Buy Order Failed: NO ID IN RESPONSE")
			logger.Printf("[AUTOSUGGEST] SENT: %v", buy)
			logger.Println()
			logger.Printf("[AUTOSUGGEST] RESP: %v", jresp)
			logger.Println()
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		newU, err := uuid.NewUUID()
		if err != nil {
			logger.Printf("[AutoSuggest] UUID Error: %s", err)
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		u, err := newU.MarshalBinary()
		if err != nil {
			logger.Printf("[AutoSuggest] UUID Marshall Error: %s", err)
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		logger.Printf("[AUTOSUGGEST] \t %v", jresp)
		_, err = LocalDB.Exec(`INSERT INTO Ledger (ID,Market,Type,Cost,Price,CoinAmount,BuyFee,ProjectedSellFee,SellPrice,Time,BuyOrderID,Status) VALUES(
                              ?,?,?,?,?,?,?,?,?,?,?,?)`, u, m, "buy", willSpendWithBuyFee, buy.Price, buy.Size, buyFee, sellFee, willSellFor/willBuyCoinAmount,
			time.Now(), jresp.ID, jresp.Status)
		if err != nil {
			logger.Printf("[AutoSuggest] DB INSERT Error: %s", err)
			logger.Printf("[AutoSuggest] ----\t----\t----\t----\r\n")
			continue
		}
		logger.Printf("[AUTO_SUGGEST] Purchased %s of %s for %s, Sell at price: %.2f", buy.Size, m, buy.Price, willSellFor/willBuyCoinAmount)
		b.ChatSay(fmt.Sprintf("[AUTO_SUGGEST] Purchased %s of %s for %s, Sell at price: %.2f", buy.Size, m, buy.Price, willSellFor/willBuyCoinAmount))
	}
}

func (b *BitProphetBot) CheckBuyFills() {
	fills := []BitProphetLedgerRecord{}
	rows, err := LocalDB.Query(`SELECT ID,Market,Type,Cost,Price,CoinAmount,BuyFee,ProjectedSellFee,SellPrice,BuyOrderID,Status FROM Ledger WHERE
										Status='pending' AND Type='buy'`)
	if err != nil {
		logger.Printf("[CheckBuyFills] DB Error: %s", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		f := BitProphetLedgerRecord{}
		err = rows.Scan(&f.ID, &f.Market, &f.Type, &f.Cost, &f.Price, &f.CoinAmount, &f.BuyFee, &f.ProjectedSellFee, &f.SellPrice, &f.BuyOrderID, &f.Status)
		if err != nil {
			logger.Printf("[CheckBuyFills] DB Scan Error: %s", err)
			return
		}
		fills = append(fills, f)
	}
	logger.Printf("[CheckBuyFills] Found %d Pending Buy Orders in Ledger", len(fills))
	for _, f := range fills {
		// may need to limit this, I think 8 per some_interval is the maximum and there is no batch check (/fills endpoint isnt what I want)
		req := api.NewSecureRequest("get_order", Config.CBVersion) // create the req
		req.Credentials.Key = Config.BPInternalAccount.AccessKey   // setup it's creds
		req.Credentials.Passphrase = Config.BPInternalAccount.PassPhrase
		req.Credentials.Secret = Config.BPInternalAccount.Secret
		req.Url += f.BuyOrderID.String
		reply, err := req.Process(logger)
		if err != nil {
			logger.Printf("[CheckBuyFills] ERROR: %s", err)
			return
		}
		resp := CoinbaseOrderResponse{}
		err = json.Unmarshal(reply, &resp)
		if err != nil {
			logger.Printf("[CheckBuyFills] UnMarshall ERROR: %s", err)
			return
		}
		if resp.Settled {
			logger.Printf("[CheckBuyFills] Found Buy Fill: %s %s [%s]", resp.ID, resp.ProductId, resp.Status)
			f.FilledBuy.Bool = true
			f.FilledBuy.Valid = true
			_, err = LocalDB.Exec(`UPDATE Ledger SET Status=?,FilledBuy=? WHERE BuyOrderID=?`, resp.Status, true, resp.ID) // this update indicates the fill
			if err != nil {
				logger.Printf("[CheckBuyFills] DB Update ERROR: %s", err)
				return
			}
			// Lets try to place it for sale at saleprice
			// sell
			sreq := api.NewSecureRequest("place_order", Config.CBVersion) // create the req
			sreq.Credentials.Key = Config.BPInternalAccount.AccessKey     // setup it's creds
			sreq.Credentials.Passphrase = Config.BPInternalAccount.PassPhrase
			sreq.Credentials.Secret = Config.BPInternalAccount.Secret
			sreq.RequestMethod = "POST"
			var sell struct {
				Size   string `json:"size"`
				Price  string `json:"price"`
				Side   string `json:"side"`
				Market string `json:"product_id"`
			}
			sell.Side = "sell"
			sell.Market = f.Market.String
			sell.Size = fmt.Sprintf("%.8f", f.CoinAmount.Float64)
			sell.Price = fmt.Sprintf("%.2f", f.SellPrice.Float64)

			sbody, err := json.Marshal(sell)
			if err != nil {
				logger.Printf("[CheckBuyFills] Buy Error: %s", err)
			}
			sreq.RequestBody = string(sbody)
			sresp, err := sreq.Process(logger) // process request
			if err != nil {
				logger.Printf("[CheckBuyFills] Buy Request Error: %s", err)
			}
			jresp := CoinbaseOrderResponse{}
			err = json.Unmarshal(sresp, &jresp)
			if err != nil {
				logger.Printf("[CheckBuyFills] Sell Response unmarshall Error: %s", err)
				logger.Printf("[CheckBuyFills] ----\t----\t----\t----\r\n")
				continue
			}
			if len(jresp.ID) < 1 {
				logger.Printf("[CheckBuyFills] Sell Order Failed: NO ID IN RESPONSE")
				logger.Printf("[CheckBuyFills] SENT: %v", sell)
				logger.Println()
				logger.Printf("[CheckBuyFills] RESP: %v", jresp)
				logger.Println()
				logger.Printf("[CheckBuyFills] ----\t----\t----\t----\r\n")
				continue
			}
			_, err = LocalDB.Exec(`UPDATE Ledger SET Status=?,SellOrderID=?,Type=? WHERE BuyOrderID=?`, jresp.Status, jresp.ID, "sell", resp.ID)
			if err != nil {
				logger.Printf("[CheckBuyFills] DB Update ERROR: %s", err)
				return
			}
			b.ChatSay(fmt.Sprintf("[CheckBuyFills] Placed Sell %s %.8f for $%.2f", f.Market.String, f.CoinAmount.Float64, f.SellPrice.Float64))
		}
	}
}

func (b *BitProphetBot) CheckSellFills() {
	fills := []BitProphetLedgerRecord{}
	rows, err := LocalDB.Query(`SELECT ID,Market,Type,Cost,Price,CoinAmount,BuyFee,ProjectedSellFee,SellPrice,SellOrderID,Status,Time FROM Ledger WHERE
										TimeSold IS NULL AND Type='sell'`)
	if err != nil {
		logger.Printf("[CheckSellFills] DB Error: %s", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		f := BitProphetLedgerRecord{}
		err = rows.Scan(&f.ID, &f.Market, &f.Type, &f.Cost, &f.Price, &f.CoinAmount, &f.BuyFee, &f.ProjectedSellFee, &f.SellPrice, &f.SellOrderID, &f.Status, &f.Time)
		if err != nil {
			logger.Printf("[CheckSellFills] DB Scan Error: %s", err)
			return
		}
		fills = append(fills, f)
	}
	logger.Printf("[CheckSellFills] Found %d Pending Sell Orders in Ledger", len(fills))
	for _, f := range fills {
		logger.Printf("[CheckSellFills] Found [%s] [%s] [%.8f] [SellPrice: $%.2f]", f.SellOrderID.String, f.Market.String, f.CoinAmount.Float64, f.SellPrice.Float64)
		req := api.NewSecureRequest("get_order", Config.CBVersion) // create the req
		req.Credentials.Key = Config.BPInternalAccount.AccessKey   // setup it's creds
		req.Credentials.Passphrase = Config.BPInternalAccount.PassPhrase
		req.Credentials.Secret = Config.BPInternalAccount.Secret
		req.Url += f.SellOrderID.String
		reply, err := req.Process(logger)
		if err != nil {
			logger.Printf("[CheckSellFills] ERROR: %s", err)
			return
		}
		resp := CoinbaseOrderResponse{}
		err = json.Unmarshal(reply, &resp)
		if err != nil {
			logger.Printf("[CheckSellFills] UnMarshall ERROR: %s", err)
			return
		}
		if resp.Settled {
			logger.Printf("[CheckSellFills] [Settled Sell] [%s] [%s] [%s] [%s] [SellPrice: %s] [DoneAt: %s]", resp.ID, resp.Status, resp.ProductId, resp.Size, resp.ExecutedValue, resp.DoneAt.String())
			// update the database to make it stop checkin this one
			_, err = LocalDB.Exec(`UPDATE Ledger SET Status=?,SoldValue=?,TimeSold=?,FilledSell=?,SellFee=? WHERE SellOrderID=?`, resp.Status, resp.ExecutedValue, resp.DoneAt, true, resp.FillFees, resp.ID)
			if err != nil {
				logger.Printf("[CheckSellFills] DB Error: %s", err)
				return
			}
			b.ChatSay(fmt.Sprintf("[CheckSellFills] [Settled Sell] [%s %s] [SellValue: %s]", resp.Size, resp.ProductId, resp.ExecutedValue))
		}
	}
}

func (b *BitProphetBot) ChatSay(text string) {
	b.AutoSuggestChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "AUTO_SUGGEST",
		EventData: text,
	}
}
