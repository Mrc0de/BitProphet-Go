package CoinbaseAPI

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SecureRequest struct {
	Url           string
	RequestName   string
	RequestMethod string
	RequestBody   string
	Client        *http.Client
	Timestamp     time.Time
	Credentials   CoinbaseCredentials
}

type CoinbaseCredentials struct {
	// PRIVATE! NEVER EXPOSE!
	Key        string
	Passphrase string
	Secret     string
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
		Timestamp:     time.Now(),
		Credentials: CoinbaseCredentials{
			Key:        "",
			Passphrase: "",
			Secret:     "",
		},
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

func (s *SecureRequest) Process(logger *log.Logger) ([]byte, error) {
	var (
		err   error
		req   *http.Request
		reply []byte
	)
	if len(s.RequestBody) < 1 {
		req, err = http.NewRequest(s.RequestMethod, "https://api.pro.coinbase.com"+s.Url, nil)
	} else {
		req, err = http.NewRequest(s.RequestMethod, "https://api.pro.coinbase.com"+s.Url, bytes.NewBuffer([]byte(s.RequestBody)))
	}
	if err != nil {
		if logger != nil {
			logger.Printf("[SecureRequest::Process] Error creating request: %s", err)
		}
		return reply, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("CB-ACCESS-KEY", s.Credentials.Key)
	req.Header.Set("CB-ACCESS-TIMESTAMP", string(s.Timestamp.Unix()))
	req.Header.Set("CB-ACCESS-PASSPHRASE", s.Credentials.Passphrase)
	// Generate the signature
	// decode Base64 secret
	sec, err := base64.StdEncoding.DecodeString(s.Credentials.Secret)
	if err != nil {
		if logger != nil {
			logger.Printf("Error decoding secret: %s", err)
		}
		return reply, err
	}
	// Create SHA256 HMAC w/ secret
	h := hmac.New(sha256.New, sec)
	// write timestamp
	h.Write([]byte(strconv.FormatInt(s.Timestamp.Unix(), 10)))
	//write method
	h.Write([]byte(s.RequestMethod))
	//write path
	h.Write([]byte(s.Url))
	//write body (if any)
	if len(s.RequestBody) > 1 {
		h.Write([]byte(s.RequestBody))
	}
	shaStr := hex.EncodeToString(h.Sum(nil))
	// encode the result to base64
	b64Signature := base64.StdEncoding.EncodeToString([]byte(shaStr))
	req.Header.Set("CB-ACCESS-SIGNATURE", b64Signature)

	// Send
	resp, err := s.Client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Printf("Error reading response: %s", err)
		}
		return reply, err
	}
	defer resp.Body.Close()

	reply, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		if logger != nil {
			logger.Printf("Error reading body: %s", err)
		}
		return reply, err
	}
	return reply, err
}
