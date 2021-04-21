package CoinbaseAPI

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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
			return "/v2/accounts"
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
			fmt.Printf("UNCAUGHT EXCEPTION: %s", r)
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
	s.Timestamp = time.Now().UTC()
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
		fmt.Printf("Error decoding secret: %s", err)
		return req, err
	}
	logger.Printf("[SecureRequest::Process] Base64 Secret: %s", s.Credentials.Secret)
	logger.Printf("[SecureRequest::Process] Decoded Secret Length: %d", len(sec))
	logger.Printf("[SecureRequest::Process] Decoded Secret: %h", sec)
	// Create SHA256 HMAC w/ secret
	h := hmac.New(sha256.New, sec)
	msg := fmt.Sprintf("%d%s%s%s%s", s.Timestamp.Unix(), s.RequestMethod, s.Url, s.RequestBody)
	logger.Printf("ENCODING: %s", msg)

	h.Write([]byte(msg))
	sha := h.Sum(nil)
	logger.Printf("[SecureRequest::Process] Encoded Signature Size: %d", len(sha))

	// encode the result to base64
	shaEnc := base64.StdEncoding.EncodeToString(sha)
	req.Header["CB-ACCESS-SIGN"] = []string{shaEnc}

	for h, v := range req.Header {
		logger.Printf("[%s] %s", h, v) // danger
	}
	return req, nil
}
