package CoinbaseAPI

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type SecureRequest struct {
	Url           string
	RequestName   string
	RequestMethod string
	RequestBody   string
	Timestamp     time.Time
	Credentials   *CoinbaseCredentials
	CBVersion     string
}

func NewSecureRequest(RequestName string, version string) *SecureRequest {
	return &SecureRequest{
		Url:           UrlForRequestName(RequestName),
		RequestName:   RequestName,
		RequestMethod: "GET", // default, change as needed
		Timestamp:     time.Now().UTC(),
		Credentials: &CoinbaseCredentials{
			Key:        "",
			Passphrase: "",
			Secret:     "",
		},
		CBVersion: version,
	}
}

type CoinbaseCredentials struct {
	// PRIVATE! NEVER EXPOSE!
	Key        string
	Passphrase string
	Secret     string
}

type CoinbaseAccount struct {
	// PRIVATE! NEVER EXPOSE directly!
	AccountID      string `json:"id"`
	Currency       string `json:"currency"`
	Balance        string `json:"balance"`
	Available      string `json:"available"`
	Hold           string `json:"hold"`
	ProfileID      string `json:"profile_id"`
	TradingEnabled bool   `json:"trading_enabled"`
}

type CoinbaseOrder struct {
	// dont expose the private parts
	Id            string    `json:"id"`
	Price         string    `json:"price"`
	Size          string    `json:"size"`
	ProductId     string    `json:"product_id"`
	Side          string    `json:"side"`
	Stp           string    `json:"stp"`
	Type          string    `json:"type"`
	TimeInForce   string    `json:"time_in_force"`
	PostOnly      bool      `json:"post_only"`
	CreatedAt     time.Time `json:"created_at"`
	FillFees      string    `json:"fill_fees"`
	FilledSize    string    `json:"filled_size"`
	ExecutedValue string    `json:"executed_value"`
	Status        string    `json:"status"`
	Settled       bool      `json:"settled"`
}

type CoinbaseFill struct {
	// dont expose the private parts
	TradeId   int       `json:"trade_id"`
	ProductId string    `json:"product_id"`
	Price     string    `json:"price"`
	Size      string    `json:"size"`
	OrderId   string    `json:"order_id"`
	CreatedAt time.Time `json:"created_at"`
	Liquidity string    `json:"liquidity"`
	Fee       string    `json:"fee"`
	Settled   bool      `json:"settled"`
	Side      string    `json:"side"`
}

type CoinbaseReport struct {
	Id          string    `json:"id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	FileUrl     string    `json:"file_url"`
	Params      struct {
		StartDate time.Time `json:"start_date"`
		EndDate   time.Time `json:"end_date"`
	} `json:"params"`
}

func UrlForRequestName(name string) string {
	switch strings.ToLower(name) {
	case "list_accounts":
		{
			return "/accounts"
		}
	case "list_orders":
		{
			return "/orders"
		}
	case "list_fills":
		{
			return "/fills"
		}
	case "report_create":
		{
			return "/reports" // returns an id
		}
	case "report_fetch":
		{
			return "/reports/:" // :reportid <-- supply this part
		}
	default:
		{
			return ""
		}
	}
}

func (s *SecureRequest) Process(logger *log.Logger) ([]byte, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("UNCAUGHT EXCEPTION: %s", r)
			debug.PrintStack()
		}
	}()
	fmt.Println("[SecureRequest::Process]")
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
	s.Timestamp = time.Now().UTC()
	// Header.Add/.Set will jack up your all-caps headers, set them directly, its just a map
	req.Header["Accept"] = []string{"application/json"}
	req.Header["CB-ACCESS-KEY"] = []string{s.Credentials.Key}
	req.Header["CB-ACCESS-TIMESTAMP"] = []string{fmt.Sprintf("%d", s.Timestamp.Unix())}
	req.Header["CB-ACCESS-PASSPHRASE"] = []string{s.Credentials.Passphrase}
	req.Header["CB-VERSION"] = []string{s.CBVersion}
	req.Header["Content-Type"] = []string{"application/json"}
	req.Header.Add("User-Agent", "BitProphet-Go 1.337")
	// Generate the signature
	// decode Base64 secret
	sec, err := base64.StdEncoding.DecodeString(s.Credentials.Secret)
	if err != nil {
		if logger != nil {
			logger.Printf("Error decoding secret: %s", err)
		}
		return reply, err
	}
	if logger != nil {
		//logger.Printf("[SecureRequest::Process] Base64 Secret: %s", s.Credentials.Secret)
		//logger.Printf("[SecureRequest::Process] Decoded Secret Length: %d", len(sec))
		//logger.Printf("[SecureRequest::Process] Decoded Secret: %x", sec)
	}
	// Create SHA256 HMAC w/ secret
	h := hmac.New(sha256.New, sec)
	var msg string
	if len(s.RequestBody) < 1 {
		msg = fmt.Sprintf("%d%s%s", s.Timestamp.Unix(), s.RequestMethod, s.Url)
	} else {
		msg = fmt.Sprintf("%d%s%s%s", s.Timestamp.Unix(), s.RequestMethod, s.Url, s.RequestBody)
	}
	if logger != nil {
		//logger.Printf("[SecureRequest::Process] ENCODING MSG")
	}
	h.Write([]byte(msg))
	sha := h.Sum(nil)
	if logger != nil {
		//logger.Printf("[SecureRequest::Process] Encoded Signature Size: %d", len(sha))
	}
	// encode the result to base64
	shaEnc := base64.StdEncoding.EncodeToString(sha)
	req.Header["CB-ACCESS-SIGN"] = []string{shaEnc}
	if logger != nil {
		//logger.Printf("[SecureRequest::Process] ENCODED MSG Size: %d", len(shaEnc))
	}

	c := &http.Client{}
	re, err := c.Do(req)
	if err != nil {
		if logger != nil {
			logger.Printf("[SecureRequest::Process] Error reading response: %s", err)
		}
		return reply, err
	}
	defer re.Body.Close()
	reply, err = ioutil.ReadAll(re.Body)
	if err != nil {
		if logger != nil {
			logger.Printf("[SecureRequest::Process] Error reading body: %s", err)
		}
		return reply, err
	}
	return reply, nil
}
