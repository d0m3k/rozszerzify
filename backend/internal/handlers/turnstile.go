package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type turnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
	Action     string   `json:"action"`
	Hostname   string   `json:"hostname"`
}

// VerifyTurnstile validates a Cloudflare Turnstile token server-side.
// remoteIP is optional (chi RealIP already rewrote RemoteAddr from
// X-Forwarded-For); passing it improves the security model but is not required.
func VerifyTurnstile(secret, token, remoteIP string) (bool, error) {
	if secret == "" {
		return false, nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return false, nil
	}

	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var tr turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return false, err
	}
	return tr.Success, nil
}
