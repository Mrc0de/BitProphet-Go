package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

// This is not part of the web side
// but it informs the web side in some cases
// tracks markets via influxdb
// performs auto trades according to selected strategies

// This 'Service' runs the 'Bot' for the 'user', so they dont have to
// The web interface and accounts system MUST require both authentication to this service AND authorization of automated use
// AND some form of auth to coinbase on the users behalf, OAUTH is a good candidate as they can revoke access easily, if desired
// make sure to detect that happening so we can stop (and notify the user)

// bpService serves the frontend needs
// BitProphetServiceClient is the internal websocket(from server to coinbase) with a reporting pipeline (leads back to callers select)
// The part that makes recommendations, and performs auto-trades for users, is the BitProphetBot 'The Bot'
type bpService struct {
	Client           *BitProphetServiceClient
	ServiceClients   []*BitProphetServiceClient
	TheBot           *BitProphetBot
	BotChannel       chan *bpServiceEvent
	ReportingChannel chan *bpServiceEvent
	CommandChannel   chan *bpServiceCommandMsg
	CoinbaseChannel  chan *bpCoinBaseMsg
	Quit             bool
}

type bpServiceEvent struct {
	Time      time.Time
	EventType string
	EventData string
}

type bpServiceCommandMsg struct {
	Time      time.Time
	Command   string
	Arguments []string
}

type bpCoinBaseMsg struct {
	Time    time.Time
	MsgType string
	MsgBody []byte
	MsgObj  CoinbaseMessage
}

func CreateBPService() *bpService {
	var bps = bpService{
		Client:           SpawnBitProphetClient(),
		ServiceClients:   make([]*BitProphetServiceClient, 0),
		TheBot:           CreateBitProphetBot(),
		ReportingChannel: make(chan *bpServiceEvent, 10), // this makes it into the webservice log
		CommandChannel:   make(chan *bpServiceCommandMsg, 0),
		CoinbaseChannel:  make(chan *bpCoinBaseMsg, 0),
		Quit:             false,
	}
	bps.Client.ParentService = &bps
	bps.ServiceClients = append(bps.ServiceClients, bps.Client)
	return &bps
}

// CLIENT
type BitProphetServiceClient struct {
	// wsClient that connects to coinbase
	// Native Authentication for host user (optional) (server side config file or envvar)
	// Public Authentication (OAUTH) for other users
	WSHost            string
	WSConn            *websocket.Conn
	HTTPResp          *http.Response
	Connected         bool
	ParentService     *bpService
	CBWriteChannel    chan string
	QuitChannel       chan bool
	Influx            influx
	IsInternalAccount bool
}

func SpawnBitProphetClient() *BitProphetServiceClient {
	var bp = BitProphetServiceClient{
		WSHost:         Config.BitProphetServiceClient.WSHost,
		WSConn:         nil,
		HTTPResp:       nil,
		Connected:      false,
		ParentService:  nil,
		CBWriteChannel: make(chan string, 0),
		QuitChannel:    make(chan bool, 0),
	}
	return &bp
}

// MSGs
type CoinbaseMessage struct {
	Type     string `json:"type"`
	Sequence int64  `json:"sequence"`
	TradeID  int64  `json:"trade_id"`
	// literally everything else is coming down as a string
	ProductID    string `json:"product_id"`
	Price        string `json:"price"`
	Open24Hour   string `json:"open_24h"`
	Volume24Hour string `json:"volume_24h"`
	Low24Hour    string `json:"low_24h"`
	High24Hour   string `json:"high_24h"`
	Volume30Day  string `json:"volume_30d"`
	BestBid      string `json:"best_bid"`
	BestAsk      string `json:"best_ask"`
	Side         string `json:"side"`
	TimeStr      string `json:"time"`
	Time         time.Time
	LastSize     string `json:"last_size"`
}

type bpCBSubscribeRequest struct {
	Type     string   `json:"type"` // subscribe
	Products []string `json:"product_ids"`
	Channels []string `json:"channels"`
	//{
	//    "type": "subscribe",
	//    "product_ids": [
	//        "ETH-USD",
	//        "ETH-EUR"
	//    ],
	//    "channels": [
	//        "level2",
	//        "heartbeat",
	//        {
	//            "name": "ticker",
	//            "product_ids": [
	//                "ETH-BTC",
	//                "ETH-USD"
	//            ]
	//        }
	//    ]
	//}
	// The ConnectCoinbase() code makes the handling more obvious (to me)
}

//
//// Misc Types for MSGs
//type bpCBPrice struct {
//	Market string
//	Bid    float64
//	Ask    float64
//	Last   float64
//}

// ////////////////////////// //
// BitProphet Service Method //
// //////////////////////// //

// bpService Run
func (b *bpService) Run() {
	b.ReportingChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "INTERNAL",
		EventData: "BitProphet Service Starting...",
	}
	err := b.Client.Influx.Connect()
	if err != nil {
		b.ReportingChannel <- &bpServiceEvent{
			Time:      time.Now(),
			EventType: "INTERNAL",
			EventData: fmt.Sprintf("[bpService] [INFLUX_CONNECT_ERROR] [%s]", err),
		}
	}
	// Internal Account (if used)
	if Config.BPInternalAccount.Enabled {
		b.ReportingChannel <- &bpServiceEvent{
			Time:      time.Now(),
			EventType: "INTERNAL",
			EventData: "BitProphet Internal Account Client Enabled!",
		}
	}
	// Bot startup
	go b.TheBot.Run()

	for {
		select {
		case event := <-b.TheBot.ServiceChannel:
			{
				b.ReportingChannel <- event
			}
		case sug := <-b.TheBot.AutoSuggestChannel:
			{
				// handle AutoSuggest Events
				b.ReportingChannel <- &bpServiceEvent{
					Time:      time.Now(),
					EventType: "INTERNAL",
					EventData: fmt.Sprintf("Bot made a suggestion: %s %s", sug.EventType, sug.EventData),
				}
			}
		case cmd := <-b.CommandChannel:
			{
				fmt.Printf("[bpService] \tCommand: \t[%s]\r\n", cmd.Command)
				// Check command, execute
				// report back
				b.ReportingChannel <- &bpServiceEvent{
					Time:      time.Now(),
					EventType: "INTERNAL",
					EventData: fmt.Sprintf("EXECUTING COMMAND: [%s]", cmd.Command),
				}
				if cmd.Command == "QUITNOW" {
					b.Quit = true
				}
			}
		case cbMsg := <-b.CoinbaseChannel:
			{
				if cbMsg.MsgObj.Type == "ticker" {
					// write ticker to influx
					err = b.Client.Influx.WriteCoinbaseTicker(cbMsg.MsgObj)
					if err != nil {
						b.ReportingChannel <- &bpServiceEvent{
							Time:      time.Now(),
							EventType: "INTERNAL",
							EventData: fmt.Sprintf("[bpService] [INFLUX_WRITE_ERROR] [%s]", err),
						}
					}
				} else {
					b.ReportingChannel <- &bpServiceEvent{
						Time:      time.Now(),
						EventType: "COINBASE",
						EventData: fmt.Sprintf("[bpService] [%s] [%s] [%s] [%s] [%s]", cbMsg.MsgObj.Type, cbMsg.MsgObj.ProductID, cbMsg.MsgObj.Price, cbMsg.MsgObj.BestAsk, cbMsg.MsgObj.BestBid),
					}
				}
			}
		}
		if b.Quit {
			b.ReportingChannel <- &bpServiceEvent{
				Time:      time.Now(),
				EventType: "INTERNAL",
				EventData: "BitProphet Service Stopping...",
			}
			break
		}
	}
}

// ///////////////////////// //
// BitProphetClient Methods //
// /////////////////////// //

//////////////////////////////////////////////
// CLIENT connect (Websocket, normal, no auth)
func (b *BitProphetServiceClient) ConnectCoinbase() error {
	var err error
	fmt.Printf("Connecting to Coinbase: wss://%s\r\n", b.WSHost)
	d := websocket.DefaultDialer
	d.TLSClientConfig = &tls.Config{
		ServerName: b.WSHost,
	}
	d.TLSClientConfig.ServerName = b.WSHost

	b.WSConn, b.HTTPResp, err = d.Dial("wss://"+b.WSHost, nil)
	if err != nil {
		return fmt.Errorf("[ConnectCoinbase] Error: %s", err)
	}
	b.Connected = true
	b.ParentService.ReportingChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "SERVICE_CLIENT",
		EventData: fmt.Sprintf("Connected to Coinbase: %s", b.WSHost),
	}
	// We are connected, subscribe to defaults
	for _, coin := range Config.BitProphetServiceClient.DefaultSubscriptions {
		logger.Printf("[ConnectCoinbase] Subscribing to [%s]", coin)
		var finalJson []string
		finalJson = append(finalJson, "ticker") // I removed heartbeat because I forgot what it was for and I dont think I need it at all, it goes here
		subMsg := bpCBSubscribeRequest{"subscribe", []string{coin}, finalJson}
		if err := b.WSConn.WriteJSON(subMsg); err != nil {
			return fmt.Errorf("[ConnectCoinbase] Subscribe Error: %s", err)
		}
	}
	logger.Printf("[ConnectCoinbase] Subscribing Complete.")
	//////////////
	go b.ReadPump()
	go b.WritePump()
	go func() {
		for {
			select {
			case qSignal := <-b.QuitChannel:
				{
					if qSignal {
						b.ParentService.ReportingChannel <- &bpServiceEvent{
							Time:      time.Now(),
							EventType: "SERVICE_CLIENT",
							EventData: "BitProphet Service Client Killed for reconnect",
						}
						break
					}
				}
				//default:
				//	{
				//		//
				//	}
			}
		}
	}()
	return err
}

func (b *BitProphetServiceClient) ReadPump() {
	defer func() {
		_ = b.WSConn.Close()
		b.Connected = false
		b.ParentService.ReportingChannel <- &bpServiceEvent{
			Time:      time.Now(),
			EventType: "SERVICE_CLIENT",
			EventData: fmt.Sprintf("WEBSOCKET DISCONNECTED"),
		}
		b.QuitChannel <- true
		if err := b.ConnectCoinbase(); err != nil {
			b.ParentService.ReportingChannel <- &bpServiceEvent{
				Time:      time.Now(),
				EventType: "SERVICE_CLIENT",
				EventData: fmt.Sprintf("RECONNECT ERROR: %s", err),
			}
		}
	}()
	for {
		_, msg, err := b.WSConn.ReadMessage()
		if err != nil {
			b.ParentService.ReportingChannel <- &bpServiceEvent{
				Time:      time.Now(),
				EventType: "SERVICE_CLIENT",
				EventData: fmt.Sprintf("READ ERROR: %s", err),
			}
			break
		}
		var obj CoinbaseMessage
		err = json.Unmarshal(msg, &obj)
		if err != nil {
			b.ParentService.ReportingChannel <- &bpServiceEvent{
				Time:      time.Now(),
				EventType: "SERVICE_CLIENT",
				EventData: fmt.Sprintf("JSON Unmarshall ERROR: %s", err),
			}
			// dont break out, just report it
		} else {
			b.ParentService.CoinbaseChannel <- &bpCoinBaseMsg{
				Time:    time.Now(),
				MsgType: "COINBASE",
				MsgBody: msg,
				MsgObj:  obj,
			}
		}
	}
}

func (b *BitProphetServiceClient) WritePump() {
	defer func() {
		_ = b.WSConn.Close()
		b.Connected = false
		b.ParentService.ReportingChannel <- &bpServiceEvent{
			Time:      time.Now(),
			EventType: "SERVICE_CLIENT",
			EventData: fmt.Sprintf("WEBSOCKET DISCONNECTED [SLEEPING 1m]"),
		}
		b.QuitChannel <- true
		time.Sleep(time.Minute * 1)
		if err := b.ConnectCoinbase(); err != nil {
			b.ParentService.ReportingChannel <- &bpServiceEvent{
				Time:      time.Now(),
				EventType: "SERVICE_CLIENT",
				EventData: fmt.Sprintf("RECONNECT ERROR: %s", err),
			}
		}
	}()
	for {
		select {
		case msg := <-b.CBWriteChannel:
			{
				err := b.WSConn.WriteMessage(websocket.TextMessage, []byte(msg))
				if err != nil {
					b.ParentService.ReportingChannel <- &bpServiceEvent{
						Time:      time.Now(),
						EventType: "SERVICE_CLIENT",
						EventData: fmt.Sprintf("WRITE ERROR: %s", err),
					}
					break
				}
			}
		}
	}
}
