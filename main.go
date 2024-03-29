package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
	"os/signal"
)

// Iteration 5,203,002 of BitProphet, Fourth language
// MrC0de@geekprojex.com

// Web Server w/ crypto services
// influx market data storage
// websocket client data
// Web Client
// Coinbase (Primary Integration)
// Account Information Display
// Automatic Trading (main feature) (very basic buy/sell profit mechanism)
// the price doesnt matter, the amplitude of change and the frequency of change does.
// it skims the slopes (totally just made that up)
var (
	// Globals
	Config     Configuration
	logger     *log.Logger
	WebService *httpService
	LocalDB    *sql.DB

	// Channels
	DebugChannel  chan string
	WWWLogChannel chan string

	// Cmdline Flags
	ConfigFile string
	Debug      bool
	Verbose    bool
)

func main() {
	flag.StringVar(&ConfigFile, "c", "BitProphet-Go.yml", "Alternate Config (Default: BitProphet-Go.yml)")
	flag.BoolVar(&Debug, "debug", false, "Most Verbose Output")
	flag.BoolVar(&Verbose, "v", false, "Verbose Output")
	flag.Parse()
	logger = log.New(os.Stdout, "", log.LstdFlags)
	err := Config.load(ConfigFile)
	if err != nil {
		logger.Printf("Error, Cannot Load Config: %s", err)
		os.Exit(1)
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[BitProphet-Go] [UNHANDLED_ERROR]: %s", r)
			os.Exit(1)
		}
	}()

	// Channels
	DebugChannel := make(chan string)
	WWWLogChannel := make(chan string)
	quitKey := make(chan os.Signal, 1)

	// Start
	wsSrv := &WebSocketService{WebServer: nil, WsHub: newWSHub()}
	WebService = &httpService{WsService: wsSrv}
	WebService.Init()
	signal.Notify(quitKey, os.Interrupt)
	mainQuit := false
	// Start up Backend Service (BitProphetService)
	LocalDB, err = sql.Open("mysql", Config.BPData.DSN)
	if err != nil {
		panic(err)
	}
	defer LocalDB.Close()
	err = LocalDB.Ping()
	if err != nil {
		panic(err)
	}
	BitProphet := CreateBPService()
	go BitProphet.Run()
	first := true
	// Loop
	for {
		select {
		case bpReport := <-BitProphet.ReportingChannel:
			{
				logger.Printf("[%s] [%s]", bpReport.EventType, bpReport.EventData)
				WebService.WsService.WsHub.Broadcast <- []byte(bpReport.EventData)
			}
		case d := <-DebugChannel:
			{
				if Debug {
					logger.Printf("[DEBUG] %s", d)
				}
			}
		case logData := <-WWWLogChannel:
			{
				logger.Printf("[WWW] %s", logData)
			}
		case <-quitKey:
			{
				// Start shutting down
				logger.Println("[BitProphet-Go] Shutting down.")
				mainQuit = true
			}
		}
		if mainQuit {
			logger.Println("[BitProphet-Go] Shutdown Finished.")
			break
		}
		if first {
			err = BitProphet.Client.ConnectCoinbase()
			if err != nil {
				logger.Printf("[ERROR] Cannot Connect Coinbase Client: \t %s", err)
				os.Exit(1)
			}
			first = false
		}
	}

}
