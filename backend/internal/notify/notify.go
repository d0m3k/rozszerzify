package notify

import (
	"log"
	"net/http"
	"net/url"
)

// Notifier sends basic Pushover notifications. No-op when not configured.
type Notifier struct {
	userKey  string
	appToken string
}

func New(userKey, appToken string) *Notifier {
	return &Notifier{userKey: userKey, appToken: appToken}
}

// Enabled reports whether both keys are configured.
func (n *Notifier) Enabled() bool {
	return n.userKey != "" && n.appToken != ""
}

// Send pushes a notification; failures are logged but never fatal.
func (n *Notifier) Send(title, message string) {
	if !n.Enabled() {
		return
	}
	form := url.Values{}
	form.Set("token", n.appToken)
	form.Set("user", n.userKey)
	form.Set("title", title)
	form.Set("message", message)
	form.Set("sound", "pushover")

	resp, err := http.PostForm("https://api.pushover.net/1/messages.json", form)
	if err != nil {
		log.Printf("[NOTIFY] %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[NOTIFY] pushover returned %d", resp.StatusCode)
	}
}