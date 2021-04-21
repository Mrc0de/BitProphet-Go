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
		Timestamp:     time.Now().UTC(),
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
	//req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CB-ACCESS-KEY", s.Credentials.Key)
	req.Header.Set("CB-ACCESS-TIMESTAMP", fmt.Sprintf("%d", s.Timestamp.Unix()))
	req.Header.Set("CB-ACCESS-PASSPHRASE", s.Credentials.Passphrase)
	// Generate the signature
	// decode Base64 secret
	sec := make([]byte, base64.StdEncoding.DecodedLen(len(s.Credentials.Secret)))
	logger.Printf("Secret: %s", s.Credentials.Secret)
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
	logger.Printf("ENCODING: %s", fmt.Sprintf("%d", s.Timestamp.Unix())+s.RequestMethod+s.Url+s.RequestBody)

	h.Write([]byte(fmt.Sprintf("%d", s.Timestamp.Unix()) + s.RequestMethod + s.Url + s.RequestBody))
	sha := make([]byte, hex.EncodedLen(h.Size()))
	num = hex.Encode(sha, h.Sum(nil))
	if logger != nil {
		logger.Printf("[SecureRequest::Process] Encoded Signature Length: %d", num)
	}
	// encode the result to base64
	//shaEnc := make([]byte, base64.StdEncoding.EncodedLen(len(sha)))
	shaEnc := base64.StdEncoding.EncodeToString(sha)
	req.Header.Set("CB-ACCESS-SIGN", string(shaEnc))
	//req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux i686; rv:10.0) Gecko/20100101 Firefox/10.0")
	for h, v := range req.Header {
		logger.Printf("[%s] %s", h, v) // danger
	}
	return req, nil
}
