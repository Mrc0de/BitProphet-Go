package main

import (
	"encoding/json"
	api "github.com/mrc0de/BitProphet-Go/CoinbaseAPI"
	"strconv"
	"strings"
	"time"
)

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
	for {
		select {
		case <-autoSuggestTicker.C:
			{
				b.ServiceChannel <- &bpServiceEvent{
					Time:      time.Now(),
					EventType: "BOT",
					EventData: "[BitProphetBot::Run] Running AutoSuggest",
				}
				b.AutoSuggest()
			}
		}
	}
}

func (b *BitProphetBot) AutoSuggest() {
	// we need to know the internal account's spendable balance
	// go slowly if possible, dont hammer anything
	// we need the 'current data' FIRST then we analyze the possibles.
	b.AutoSuggestChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "AUTO_SUGGEST",
		EventData: "START",
	}
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
			continue
		}
		logger.Printf("[AutoSuggest] MaxBuy: $%.2f", Config.BotDefaults.MaxUSDBuy)
		if minPriceBuy > Config.BotDefaults.MaxUSDBuy {
			logger.Printf("[AutoSuggest] MinPrice is more than MaxBuy, Aborting.")
			continue
		}
		// We have enough to minimum buy....
		// but instead we will buy UP TO maxBuy OR just min...
		if availCash <= Config.BotDefaults.MaxUSDBuy {
			// we cant buy up to max, just go with min
			willSpend = minPriceBuy
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
				continue
			}
		}
		logger.Printf("[AutoSuggest] Coin Amount: %.8f @ Price: $%.2f For $%.2f ( w/Fee: $%.2f )", willBuyCoinAmount, coinAsk, willSpend, willSpendWithBuyFee)
		// but SHOULD we buy now at current price?
		// Determine time-frame-price-range
		// determine buffer zone
		// is price within buffer, if so, check sell price probability, if good, purchase and immediately place for sale at sellprice
		// sellprice determined by fees, buy amount and price, as well as, percent profit setting
		logger.Printf("[AutoSuggest] Analyzing Price History for %s", m)
		//  SELECT min(price) as mini ,max(price) as maxi FROM tickers where market='LTC-USD' and time > now()-4h;
		pr, err := b.ParentService.Client.Influx.GetMinMaxPrices(m, 4)
		if err != nil {
			logger.Printf("[AutoSuggest] ERROR: %s, Aborting", err)
			continue
		}
		logger.Printf("[AutoSuggest] Price Range (4h): $%.2f - $%.2f", pr.MinPrice, pr.MaxPrice)
		// find BUFFERZONE FLOOR (above 10% above floor)
		// find BUFFERZONE CEILING (BELOW 10% below ceiling)
		zoneFloor := (pr.MinPrice * 0.10) + pr.MinPrice
		zoneRoof := pr.MaxPrice - (pr.MaxPrice * 0.10)
		logger.Printf("[AutoSuggest] Buy Buffer: $%.2f - $%.2f", zoneFloor, zoneRoof)
	}

	///////////////////////////////////////////////////////////////////////////////////////////
	// choose hour range (was 8 hours)
	// use influx instead of this.....

	//QString highPrice = findHighestValue(lastPriceRange);
	//QString highAsk = findHighestValue(askRange);
	//QString highBid = findHighestValue(bidRange);
	//
	//QString lowPrice = findLowestValue(lastPriceRange);
	//QString lowAsk = findLowestValue(askRange);
	//QString lowBid = findLowestValue(bidRange);
	//sayGdaxAutoTrader("# High Ask: $" + highAsk,currCoin);
	//sayGdaxAutoTrader("# Low Ask: $" + lowAsk,currCoin);
	//sayGdaxAutoTrader("# High Bid: $" + highBid,currCoin);
	//sayGdaxAutoTrader("# Low Bid: $" + lowBid,currCoin);
	//sayGdaxAutoTrader("# High: $" + highPrice,currCoin);
	//sayGdaxAutoTrader("# Low: $" + lowPrice,currCoin);
	// determine stuff like price/ask/bid
	//sayGdaxAutoTrader("# Price: $" + curPrice,currCoin );
	//sayGdaxAutoTrader("# Ask: $" + curAsk,currCoin );
	//sayGdaxAutoTrader("# Bid: $" + curBid,currCoin );
	//sayGdaxAutoTrader("# minUSDBuy: $" + QString().setNum(mMinUSDBuyAmount),currCoin );
	//sayGdaxAutoTrader("# minCryptoBuy: " + QString().setNum(mMinCryptoBuyAmount),currCoin );
	//sayGdaxAutoTrader("# Available: " + USDBalance,currCoin);
	//if ( USDBalance.toDouble() < mMinUSDBuyAmount  ) {
	//	sayGdaxAutoTrader("# Available $USD too low (< $"+QString().setNum(mMinUSDBuyAmount)+")",currCoin);
	//	continue;
	//}
	//QString howMuchToSpend("0.00");
	//if ( USDBalance.toDouble() > mMaxUSDBuyAmount && ((mMaxUSDBuyAmount) / curBid.toDouble()) > mMinCryptoBuyAmount )  {
	//	howMuchToSpend = QString().setNum(mMaxUSDBuyAmount - (mMaxUSDBuyAmount * 0.005));
	//} else if ( USDBalance.toDouble() < mMaxUSDBuyAmount && USDBalance.toDouble() >= mMinUSDBuyAmount && ((USDBalance.toDouble()) / curBid.toDouble()) > mMinCryptoBuyAmount )  {
	//	howMuchToSpend = QString().setNum(USDBalance.toDouble() - (USDBalance.toDouble() * 0.005));
	//} else {
	//	sayGdaxAutoTrader("# Available $USD too low For MinCryptoBuy(< "+QString().setNum(mMinCryptoBuyAmount)+")",currCoin);
	//	break;
	//}
	////shave off more than .00
	//if ( howMuchToSpend.indexOf(".",0) != -1 ) {
	//	//it has at least one deci
	//	QString pre=howMuchToSpend.mid(0,howMuchToSpend.indexOf(".",0));
	//	QString post= howMuchToSpend.mid(howMuchToSpend.indexOf(".",0)+1,2);
	//	howMuchToSpend = pre + "." + post;
	//}
	//sayGdaxAutoTrader("Can Buy " + QString().setNum(howMuchToSpend.toDouble() / curBid.toDouble()) + " Of " + currCoin + " for $" + howMuchToSpend,currCoin );
	//sayGdaxAutoTrader("# Allocated $" + howMuchToSpend +" For " + currCoin,currCoin);
	//sayGdaxAutoTrader("#################",currCoin);
	//sayGdaxAutoTrader("# Analyzing Price History",currCoin);
	//sayGdaxAutoTrader("# Coin: " + currCoin,currCoin);
	// NOW FIND THE BUFFER ZONE

}
