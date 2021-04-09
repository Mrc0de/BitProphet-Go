package main

import (
	"crypto/tls"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

// This is not part of the web side
// but it informs the web side in some cases
// tracks markets via influxdb
// performs auto trades according to selected strategies

// This 'Service' runs the 'client' for the 'user', so they dont have to
// The web interface and accounts system MUST require both authentication to this service, authorization of automated use
// AND some form of auth to coinbase on the users behalf, OAUTH is a good candidate as they can revoke access easily, if desired
// make sure to detect that happening so we can stop (and notify the user)
type bpService struct {
	Client           *BitProphetClient
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
}

type BitProphetClient struct {
	// wsClient that connects to coinbase
	// Native Authentication for host user (optional) (server side config file or envvar)
	// Public Authentication (OAUTH) for other users
	ServiceRoster  []string // need user object, roster of users using BPService
	OutMsgQueue    []string // need msg object, queue outgoing WS requests to prevent flooding coinbase, establish acceptable msg per second
	WSHost         string
	WSConn         *websocket.Conn
	HTTPResp       *http.Response
	Connected      bool
	ParentService  *bpService
	CBWriteChannel chan string
	QuitChannel    chan bool
}

func CreateBPService() *bpService {
	var bps = bpService{
		Client:           SpawnBitProphetClient(),
		ReportingChannel: make(chan *bpServiceEvent, 100),
		CommandChannel:   make(chan *bpServiceCommandMsg, 0),
		CoinbaseChannel:  make(chan *bpCoinBaseMsg, 0),
		Quit:             false,
	}
	bps.Client.ParentService = &bps
	return &bps
}

func SpawnBitProphetClient() *BitProphetClient {
	var bp = BitProphetClient{
		ServiceRoster:  make([]string, 0),
		OutMsgQueue:    make([]string, 0),
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

// Service Run
func (b *bpService) Run() {
	b.ReportingChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "INTERNAL",
		EventData: "BitProphet Service Starting...",
	}
	for {
		select {
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
				fmt.Printf("[bpService] \t[Coinbase MSG] \t[%s]\r\n", cbMsg.MsgBody)
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

// CLIENT connect
func (b *BitProphetClient) ConnectCoinbase() error {
	var err error
	d := websocket.DefaultDialer
	d.TLSClientConfig = &tls.Config{
		ServerName: b.WSHost,
	}
	d.TLSClientConfig.ServerName = b.WSHost
	b.ParentService.ReportingChannel <- &bpServiceEvent{
		Time:      time.Now(),
		EventType: "SERVICE_CLIENT",
		EventData: fmt.Sprintf("Connecting to Coinbase: wss://%s", b.WSHost),
	}
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

func (b *BitProphetClient) ReadPump() {
	defer func() {
		b.WSConn.Close()
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
		b.ParentService.CoinbaseChannel <- &bpCoinBaseMsg{
			Time:    time.Now(),
			MsgType: "COINBASE",
			MsgBody: msg,
		}
	}
}

func (b *BitProphetClient) WritePump() {
	defer func() {
		b.WSConn.Close()
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
