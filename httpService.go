package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/mrc0de/BitProphet-Go/CoinbaseAPI"
	api "github.com/mrc0de/BitProphet-Go/CoinbaseAPI" //shit like this is why we cant have nice things....
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"time"
)

type httpService struct {
	HTTPServer        *http.Server
	HTTPServerChannel chan *httpServiceEvent
	WsService         *WebSocketService
	Quit              bool
}

type httpServiceEvent struct {
	Time      time.Time
	RemoteIP  net.IP
	EventType string
	EventData string
}

func (h *httpService) Init() {
	defer func() {
		if r := recover(); r != nil {
			logger.Printf("[httpService.Init] [UNHANDLED_ERROR]: %s", r)
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	// HTTP(s) Service
	logger.Printf("[httpService.Init] Starting WWW Service [HTTPS]")
	r := mux.NewRouter().StrictSlash(true)
	// routes
	r.HandleFunc("/", WWWHome).Methods("GET")
	r.HandleFunc("/stats/user/default", processTimeout(InternalUserStats, 10*time.Second)).Methods("GET")
	//r.HandleFunc("/stats/market/{id}", WWWHome).Methods("POST")
	r.Use(h.LogRequest)

	// Websocket Setup
	h.WsService.WebServer = h
	go h.WsService.WsHub.run()
	r.HandleFunc("/ws", func(w2 http.ResponseWriter, r2 *http.Request) {
		websocketUpgrade(h.WsService.WsHub, w2, r2)
	})

	logger.Printf("%v", h.HTTPServer)
	h.HTTPServer = &http.Server{
		Addr:         Config.Web.Listen + ":443",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	go func() {
		if err := h.HTTPServer.ListenAndServeTLS(Config.Web.CertFile, Config.Web.KeyFile); err != nil {
			logger.Printf("[httpService.Init] %s", err)
		}
	}()
	logger.Println("[httpService.Init] [Started HTTPS]")

	// Start http redirect to https
	go func() {
		if err := http.ListenAndServe("0.0.0.0:80", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			logger.Printf("[%s] Redirecting %s from HTTP to HTTPS", r.RequestURI, r.RemoteAddr)
			http.Redirect(rw, r, "https://"+r.Host+r.URL.String(), http.StatusFound) // 302 doesnt get cached
		})); err != nil {
			logger.Printf("[StartRedirectToHTTPS] %s", err)
		}
	}()

}

// request logger
func (h *httpService) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("[Request] [URI: %s] [Host: %s] [Len: %d] [RemoteAddr: %s] \r\n\t\t\t[UserAgent: %s]",
			r.RequestURI, r.Host, r.ContentLength, r.RemoteAddr, r.UserAgent())
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Printf("[Request] Error Reading Body: %s", err)
		} else if len(body) != 0 {
			logger.Printf("[Request] [Body:%s]", body)
		}
		next.ServeHTTP(w, r)
	})
}

// route handlers
func WWWHome(w http.ResponseWriter, r *http.Request) {
	var data struct {
		WsHost string
	}
	data.WsHost = r.Host
	logger.Printf("[%s] %s %s", r.RequestURI, r.Method, r.RemoteAddr)
	tmpl := template.Must(template.ParseFiles(Config.Web.Path+"/home.tmpl", Config.Web.Path+"/base.tmpl"))
	err := tmpl.Execute(w, &data)
	if err != nil {
		logger.Printf("Error Parsing Template: %s", err)
	}
}

func processTimeout(h http.HandlerFunc, duration time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), duration)
		defer cancel()
		r = r.WithContext(ctx)
		processDone := make(chan bool)
		go func() {
			h(w, r)
			processDone <- true
		}()
		select {
		case <-ctx.Done():
			w.Write([]byte(`{"error": "process timeout"}`))
		case <-processDone:
		}
	}
}

/////////////////////////////
// src: https://docs.pro.coinbase.com
////////////////////////////////////////////////////////////////
//All REST requests must contain the following headers:
//
//CB-ACCESS-KEY The api key as a string.
//CB-ACCESS-SIGN The base64-encoded signature (see Signing a Message).
//CB-ACCESS-TIMESTAMP A timestamp for your request.
//CB-ACCESS-PASSPHRASE The passphrase you specified when creating the API key.
//
//All request bodies should have content type application/json and be valid JSON.

// The CB-ACCESS-SIGN header is generated by creating a sha256 HMAC using the base64-decoded secret key
// on the prehash string timestamp + method + requestPath + body (where + represents string concatenation)
// and base64-encode the output.
//
//The timestamp value is the same as the CB-ACCESS-TIMESTAMP header.
//The body is the request body string or omitted if there is no request body (typically for GET requests).
//The method should be UPPER CASE.
////////////////////////////////////////////////////////////////

func InternalUserStats(w http.ResponseWriter, r *http.Request) {
	// fetch stats for internal user
	// /stats/user/default
	// PUBLIC!! DO NOT OUTPUT ANY SENSITIVE DATA!!!
	if !Config.BPInternalAccount.Enabled {
		http.Error(w, fmt.Sprintf("Not Allowed"), http.StatusForbidden)
		return
	}
	// Buys and matching sells with brief activity analysis
	// Transaction Steps:
	// Get the keys and secret and passphrase (from somewhere, config, env, etc, using config file for starters)
	// Determine the URL and the body contents (if any)
	// take timestamp of now
	// Produce Signature
	// Write Headers (w/ signature etc)
	// Write body (if any, most have none)
	// Post and fetch reply
	// Clean reply and ONLY RETURN NON SENSITIVE DATA to users
	// By default, this is the ONLY account that 'brags' but could be enabled on other users, if desired
	// If this works well enough, I might never make other users muuuhahahahaha
	///////////////////////////////////////////////////////////////////////////
	logger.Printf("[PUBLIC]   [InternalUserStats]")
	req := api.NewSecureRequest("list_accounts")             // create the req
	req.Credentials.Key = Config.BPInternalAccount.AccessKey // setup it's creds
	req.Credentials.Passphrase = Config.BPInternalAccount.PassPhrase
	req.Credentials.Secret = Config.BPInternalAccount.Secret
	request, err := req.Process(logger) // process request
	logger.Println("Exited PROCESS")
	c := &http.Client{}
	re, err := c.Do(request)
	if err != nil {
		logger.Printf("Error reading response: %s", err)
	}
	defer re.Body.Close()

	reply, err := ioutil.ReadAll(re.Body)
	if err != nil {
		logger.Printf("Error reading body: %s", err)
	}

	logger.Printf("RESP: %v \t ------ \tE:\t %s", reply, err)
	json.NewEncoder(w).Encode(`"ERROR":"goaway"`)
}
