package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier struct {
	NotifyConfig
}

func NewNotifier(cfg NotifyConfig) (*Notifier, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Notifier{
		cfg,
	}, nil
}

func (n *Notifier) Notify(ctx context.Context, event, msg string) error {
	if len(n.Triggers) == 0 {
		return nil
	}
	trigger := NewNotifyType(event)
	if trigger == NotifyUnknown {
		return fmt.Errorf("unknown notification trigger: %v", event)
	}
	dest := n.Triggers[trigger]
	if dest == "" {
		ok := false
		dest, ok = n.Triggers[NotifyDefault]
		if !ok {
			// no configured trigger and no default set, for now don't notify
			return nil
		}
	}
	if err := sendMessage(ctx, dest, msg); err != nil {
		return err
	}
	return nil
}

type NotifyPayload struct {
	Text string `json:"text"`
}

func sendMessage(ctx context.Context, dest, msg string) error {
	marshaledPayload, err := json.Marshal(NotifyPayload{msg})
	if err != nil {
		return fmt.Errorf("failed to marshal payload to JSON: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dest, bytes.NewBuffer(marshaledPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second} // TODO make configurable
	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	return nil
}
