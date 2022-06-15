package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ingestURL = "https://%s.logicmonitor.com/rest"

const REGEX_COMPANY_NAME = "^[a-zA-Z0-9_.\\-]+$"

type Lmv1Token struct {
	AccessID  string
	Signature string
	Epoch     time.Time
}

func GetToken(method, resourcePath string, body []byte) string {
	accessID := os.Getenv("LM_ACCESS_ID")
	if accessID == "" {
		accessID = os.Getenv("LOGICMONITOR_ACCESS_ID")
	}
	accessKey := os.Getenv("LM_ACCESS_KEY")
	if accessKey == "" {
		accessKey = os.Getenv("LOGICMONITOR_ACCESS_KEY")
	}
	bearerToken := os.Getenv("LM_BEARER_TOKEN")
	if bearerToken == "" {
		bearerToken = os.Getenv("LOGICMONITOR_BEARER_TOKEN")
	}
	var token string
	if accessID != "" && accessKey != "" {
		token = generateLMv1Token(method, accessID, accessKey, body, resourcePath).String()
	} else if bearerToken != "" {
		token = "Bearer " + bearerToken
	}
	// } else {
	// 	if collToken != "" {
	// 		token = collToken
	// 	} else {
	// 		return "", fmt.Errorf("Authenticate must provide environment variable `LM_ACCESS_ID` and `LM_ACCESS_KEY` OR `LM_BEARER_TOKEN`")
	// 	}
	// }
	return token
}

func (t *Lmv1Token) String() string {
	builder := strings.Builder{}
	append := func(s string) {
		if _, err := builder.WriteString(s); err != nil {
			panic(err)
		}
	}
	append("LMv1 ")
	append(t.AccessID)
	append(":")
	append(t.Signature)
	append(":")
	append(strconv.FormatInt(t.Epoch.UnixNano()/1000000, 10))

	return builder.String()
}

//generateLMv1Token generate LMv1Token
func generateLMv1Token(method string, accessID string, accessKey string, body []byte, resourcePath string) *Lmv1Token {

	epochTime := time.Now()
	epoch := strconv.FormatInt(epochTime.UnixNano()/1000000, 10)

	methodUpper := strings.ToUpper(method)

	h := hmac.New(sha256.New, []byte(accessKey))

	writeOrPanic := func(bs []byte) {
		if _, err := h.Write(bs); err != nil {
			panic(err)
		}
	}
	writeOrPanic([]byte(methodUpper))
	writeOrPanic([]byte(epoch))
	if body != nil {
		writeOrPanic(body)
	}
	writeOrPanic([]byte(resourcePath))

	hash := h.Sum(nil)
	hexString := hex.EncodeToString(hash)
	signature := base64.StdEncoding.EncodeToString([]byte(hexString))
	return &Lmv1Token{
		AccessID:  accessID,
		Signature: signature,
		Epoch:     epochTime,
	}
}

func URL() (string, error) {
	company := os.Getenv("LM_ACCOUNT")
	if company == "" {
		if company = os.Getenv("LOGICMONITOR_ACCOUNT"); company == "" {
			return "", fmt.Errorf("Environment variable `LM_ACCOUNT` or `LOGICMONITOR_ACCOUNT` must be provided")
		}
	}
	if company != "" {
		match, _ := regexp.MatchString(REGEX_COMPANY_NAME, company)
		if !match {
			return "", fmt.Errorf("Invalid Company Name")
		}
	}
	return fmt.Sprintf(ingestURL, company), nil
}
