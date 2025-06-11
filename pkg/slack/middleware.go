package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SlackMiddleware struct {
	signingSecret string
}

func NewSlackMiddleware(signingSecret string) *SlackMiddleware {
	return &SlackMiddleware{
		signingSecret: signingSecret,
	}
}

func (m *SlackMiddleware) VerifyRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamp := r.Header.Get("X-Slack-Request-Timestamp")
		signature := r.Header.Get("X-Slack-Signature")

		if timestamp == "" || signature == "" {
			http.Error(w, "missing slack headers", http.StatusUnauthorized)
			return
		}

		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			http.Error(w, "invalid timestamp", http.StatusBadRequest)
			return
		}

		if time.Now().Unix()-ts > 60*5 {
			http.Error(w, "request too old", http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))

		baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))

		h := hmac.New(sha256.New, []byte(m.signingSecret))
		h.Write([]byte(baseString))
		computedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(computedSignature)) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			if parseErr := r.ParseForm(); parseErr == nil {
				userID := r.FormValue("user_id")
				userName := r.FormValue("user_name")
				if userID != "" {
					r.Header.Set("X-Slack-User-Id", userID)
					r.Header.Set("X-Slack-User-Name", userName)
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
