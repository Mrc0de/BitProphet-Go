package main

import (
	"encoding/json"
	api "github.com/mrc0de/BitProphet-Go/CoinbaseAPI"
	"time"
)

type BitProphetBot struct {
	ServiceChannel     chan *bpServiceEvent
	AutoSuggestChannel chan *bpServiceEvent // text only (debug)
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
	//logger.Printf("REPLY: %s", reply)
	var accList []api.CoinbaseAccount
	err = json.Unmarshal(reply, &accList)
	if err != nil {
		logger.Printf("[AutoSuggest] ERROR: %s", err)
		return
	}

	logger.Printf("[AutoSuggest] Found %d Accounts", len(accList))
	var NativeAcc api.CoinbaseAccount
	for _, acc := range accList {
		if acc.Currency == Config.BPInternalAccount.NativeCurrency {
			NativeAcc = acc

		}
	}
	logger.Printf("[AutoSuggest] Found [%s] Account Total: $%s Available: $%s", NativeAcc.Currency, NativeAcc.Balance, NativeAcc.Available)

	// at start of suggestCheck we had AutoGdaxMinCryptoBuy, AutoGdaxMinUSDBuy, and AutoGdaxMaxUSDBuy and AutoGdaxMinPercentProfit
	// Get those now
	// those came from the UI before, set them in config and read in here
	// old minimum was //min LTC Buy is 0.01 LTC // 0.01 of said coin, I think its 0.1 now, figure this out
	// choose hour range (was 8 hours)

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
