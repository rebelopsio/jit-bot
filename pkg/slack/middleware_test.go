package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewSlackMiddleware(t *testing.T) {
	signingSecret := "test-secret"
	middleware := NewSlackMiddleware(signingSecret)

	if middleware == nil {
		t.Fatal("NewSlackMiddleware returned nil")
	}

	if middleware.signingSecret != signingSecret {
		t.Errorf("Expected signing secret %s, got %s", signingSecret, middleware.signingSecret)
	}
}

func TestVerifyRequest(t *testing.T) {
	signingSecret := "test-secret"
	middleware := NewSlackMiddleware(signingSecret)

	// Test handler that sets a flag when called
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Create valid request
	body := "token=test&user_id=U123456&user_name=testuser"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Generate valid signature
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(signingSecret))
	h.Write([]byte(baseString))
	signature := "v0=" + hex.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/slack/commands", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	rr := httptest.NewRecorder()

	// Test valid request
	middleware.VerifyRequest(testHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if !handlerCalled {
		t.Error("Handler should have been called")
	}

	// Verify user ID was extracted
	if req.Header.Get("X-Slack-User-ID") != "U123456" {
		t.Errorf("Expected user ID U123456, got %s", req.Header.Get("X-Slack-User-ID"))
	}
}

func TestVerifyRequestMissingHeaders(t *testing.T) {
	middleware := NewSlackMiddleware("test-secret")
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	tests := []struct {
		name      string
		timestamp string
		signature string
		expected  int
	}{
		{"missing timestamp", "", "v0=signature", http.StatusUnauthorized},
		{"missing signature", "1234567890", "", http.StatusUnauthorized},
		{"missing both", "", "", http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/slack/commands", strings.NewReader("test=body"))
			if test.timestamp != "" {
				req.Header.Set("X-Slack-Request-Timestamp", test.timestamp)
			}
			if test.signature != "" {
				req.Header.Set("X-Slack-Signature", test.signature)
			}

			rr := httptest.NewRecorder()
			middleware.VerifyRequest(testHandler).ServeHTTP(rr, req)

			if rr.Code != test.expected {
				t.Errorf("Expected status %d, got %d", test.expected, rr.Code)
			}
		})
	}
}

func TestVerifyRequestInvalidTimestamp(t *testing.T) {
	middleware := NewSlackMiddleware("test-secret")
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	req := httptest.NewRequest("POST", "/slack/commands", strings.NewReader("test=body"))
	req.Header.Set("X-Slack-Request-Timestamp", "invalid")
	req.Header.Set("X-Slack-Signature", "v0=signature")

	rr := httptest.NewRecorder()
	middleware.VerifyRequest(testHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVerifyRequestOldTimestamp(t *testing.T) {
	middleware := NewSlackMiddleware("test-secret")
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	// Use timestamp from 10 minutes ago
	oldTimestamp := strconv.FormatInt(time.Now().Unix()-600, 10)

	req := httptest.NewRequest("POST", "/slack/commands", strings.NewReader("test=body"))
	req.Header.Set("X-Slack-Request-Timestamp", oldTimestamp)
	req.Header.Set("X-Slack-Signature", "v0=signature")

	rr := httptest.NewRecorder()
	middleware.VerifyRequest(testHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestVerifyRequestInvalidSignature(t *testing.T) {
	middleware := NewSlackMiddleware("test-secret")
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	body := "test=body"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	req := httptest.NewRequest("POST", "/slack/commands", strings.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", "v0=invalid_signature")

	rr := httptest.NewRecorder()
	middleware.VerifyRequest(testHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestVerifyRequestUserExtraction(t *testing.T) {
	signingSecret := "test-secret"
	middleware := NewSlackMiddleware(signingSecret)

	var extractedUserID, extractedUserName string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedUserID = r.Header.Get("X-Slack-User-ID")
		extractedUserName = r.Header.Get("X-Slack-User-Name")
		w.WriteHeader(http.StatusOK)
	})

	// Create form data with user info
	formData := url.Values{}
	formData.Set("token", "test-token")
	formData.Set("user_id", "U123456")
	formData.Set("user_name", "testuser")
	formData.Set("command", "/jit")
	body := formData.Encode()

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Generate valid signature
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(signingSecret))
	h.Write([]byte(baseString))
	signature := "v0=" + hex.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/slack/commands", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	rr := httptest.NewRecorder()
	middleware.VerifyRequest(testHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if extractedUserID != "U123456" {
		t.Errorf("Expected user ID U123456, got %s", extractedUserID)
	}

	if extractedUserName != "testuser" {
		t.Errorf("Expected user name testuser, got %s", extractedUserName)
	}
}
