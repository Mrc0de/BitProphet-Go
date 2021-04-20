package CoinbaseAPI

import (
	"net/http"
	"strings"
)

type SecureRequest struct {
	Url           string
	RequestName   string
	RequestMethod string
	Client        *http.Client
}

type CoinbaseCredentials struct {
	// PRIVATE! NEVER EXPOSE!

}

type CoinbaseAccount struct {
	// PRIVATE! NEVER EXPOSE directly!
	AccountID      string  `json:"id"`
	Currency       string  `json:"currency"`
	Balance        float64 `json:"balance"`
	Available      float64 `json:"available"`
	Hold           float64 `json:"hold"`
	ProfileID      string  `json:"profile_id"`
	TradingEnabled bool    `json:"trading_enabled"`
}

func NewSecureRequest(RequestName string) *SecureRequest {
	return &SecureRequest{
		Url:           UrlForRequestName(RequestName),
		RequestName:   RequestName,
		RequestMethod: "GET", // default, change as needed
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy:                  nil,
				DialContext:            nil,
				Dial:                   nil,
				DialTLSContext:         nil,
				DialTLS:                nil,
				TLSClientConfig:        nil,
				TLSHandshakeTimeout:    0,
				DisableKeepAlives:      false,
				DisableCompression:     false,
				MaxIdleConns:           0,
				MaxIdleConnsPerHost:    0,
				MaxConnsPerHost:        0,
				IdleConnTimeout:        0,
				ResponseHeaderTimeout:  0,
				ExpectContinueTimeout:  0,
				TLSNextProto:           nil,
				ProxyConnectHeader:     nil,
				MaxResponseHeaderBytes: 0,
				WriteBufferSize:        0,
				ReadBufferSize:         0,
				ForceAttemptHTTP2:      false,
			},
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       5,
		},
	}
}

func UrlForRequestName(name string) string {
	switch strings.ToLower(name) {
	case "list_accounts":
		{
			return "/accounts"
		}
	default:
		{
			return ""
		}
	}
}
