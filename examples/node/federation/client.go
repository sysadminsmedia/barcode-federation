package federation

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/crypto"
)

type Client struct {
	NodeKey   ed25519.PrivateKey
	KeyID     string
	HTTPClient *http.Client
}

func NewClient(nodeKey ed25519.PrivateKey, keyID string) *Client {
	return &Client{
		NodeKey: nodeKey,
		KeyID:   keyID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) PostToInbox(inboxURL string, messageJSON []byte) error {
	req, err := http.NewRequest("POST", inboxURL, bytes.NewReader(messageJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := crypto.SignRequest(req, c.KeyID, c.NodeKey); err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("federation delivery success", "inbox", inboxURL, "status", resp.StatusCode)
		return nil
	}
	return fmt.Errorf("inbox returned %d: %s", resp.StatusCode, string(body))
}

func (c *Client) FetchNodeMeta(metaURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", metaURL, nil)
	if err != nil {
		return nil, err
	}
	if err := crypto.SignRequest(req, c.KeyID, c.NodeKey); err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
