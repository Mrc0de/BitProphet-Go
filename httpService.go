package main

import (
	"github.com/gorilla/mux"
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

// route handlers
func WWWHome(w http.ResponseWriter, r *http.Request) {
	var data struct {
		WsHost string
	}
	data.WsHost = r.Host
	logger.Printf("[%s] %s %s", r.RequestURI, r.Method, r.RemoteAddr)
	tmpl := template.Must(template.ParseFiles(Config.Web.Path+"/Home.tmpl", Config.Web.Path+"/Base.tmpl"))
	err := tmpl.Execute(w, &data)
	if err != nil {
		logger.Printf("Error Parsing Template: %s", err)
	}
}

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
