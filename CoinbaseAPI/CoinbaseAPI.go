package CoinbaseAPI

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
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
}

func NewSecureRequest(RequestName string) *SecureRequest {
	return &SecureRequest{
		Url:           UrlForRequestName(RequestName),
		RequestName:   RequestName,
		RequestMethod: "GET", // default, change as needed
		Timestamp:     time.Now(),
		Credentials: &CoinbaseCredentials{
			Key:        "",
			Passphrase: "",
			Secret:     "",
		},
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
	AccountID      string  `json:"id"`
	Currency       string  `json:"currency"`
	Balance        float64 `json:"balance"`
	Available      float64 `json:"available"`
	Hold           float64 `json:"hold"`
	ProfileID      string  `json:"profile_id"`
	TradingEnabled bool    `json:"trading_enabled"`
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

func (s *SecureRequest) Process(logger *log.Logger) (*http.Request, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Print("UNCAUGHT EXCEPTION: %s", r)
			debug.PrintStack()
		}
	}()
	fmt.Println("[SecureRequest::Process]")
	var (
		err error
		req *http.Request
	)
	if len(s.RequestBody) < 1 {
		req, err = http.NewRequest(s.RequestMethod, "https://api.pro.coinbase.com"+s.Url, nil)
	} else {
		req, err = http.NewRequest(s.RequestMethod, "https://api.pro.coinbase.com"+s.Url, bytes.NewBuffer([]byte(s.RequestBody)))
	}
	if err != nil {
		fmt.Printf("[SecureRequest::Process] Error creating request: %s", err)
		return req, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CB-ACCESS-KEY", s.Credentials.Key)
	req.Header.Set("CB-ACCESS-TIMESTAMP", string(s.Timestamp.Unix()))
	req.Header.Set("CB-ACCESS-PASSPHRASE", s.Credentials.Passphrase)
	// Generate the signature
	// decode Base64 secret
	sec := make([]byte, 64)
	fmt.Printf("Secret Len: %d", len(s.Credentials.Secret))
	num, err := base64.StdEncoding.Decode(sec, []byte(s.Credentials.Secret))
	if err != nil {
		fmt.Printf("Error decoding secret: %s", err)
		return req, err
	}
	if logger != nil {
		logger.Printf("[SecureRequest::Process] Decoded Secret Length: %d", num)
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
	sha := make([]byte, 64)
	num = hex.Encode(sha, h.Sum(nil))
	if logger != nil {
		logger.Printf("[SecureRequest::Process] Encoded Signature Length: %d", num)
	}
	// encode the result to base64
	shaEnc := make([]byte, 64)
	base64.StdEncoding.Encode(shaEnc, sha)
	req.Header.Set("CB-ACCESS-SIGN", string(sha))

	// Send
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux i686; rv:10.0) Gecko/20100101 Firefox/10.0")
	return req, nil
}
