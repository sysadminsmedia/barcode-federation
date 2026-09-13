package model

import "encoding/json"

type FederationMessage struct {
	FBS         string          `json:"fbs"`
	Type        string          `json:"type"`
	ID          string          `json:"id"`
	Sender      string          `json:"sender"`
	Recipient   string          `json:"recipient"`
	Activity    string          `json:"activity"`
	Object      json.RawMessage `json:"object"`
	PublishedAt string          `json:"publishedAt"`
	Signature   string          `json:"signature"`
}

type BarcodeRecordRef struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	CBI           string `json:"cbi"`
	SuccessionID  string `json:"successionId,omitempty"`
}

type OutboxPage struct {
	FBS          string              `json:"fbs"`
	Type         string              `json:"type"`
	NodeID       string              `json:"nodeId"`
	TotalItems   int                 `json:"totalItems"`
	Page         int                 `json:"page"`
	PageSize     int                 `json:"pageSize"`
	NextPage     string              `json:"nextPage,omitempty"`
	PreviousPage string              `json:"previousPage,omitempty"`
	Items        []json.RawMessage   `json:"items"`
}

type OutboxRow struct {
	ID           string `db:"id"`
	ActivityType string `db:"activity_type"`
	MessageJSON  string `db:"message_json"`
	PublishedAt  string `db:"published_at"`
}

type InboxRow struct {
	ID           string `db:"id"`
	Sender       string `db:"sender"`
	ActivityType string `db:"activity_type"`
	MessageJSON  string `db:"message_json"`
	ReceivedAt   string `db:"received_at"`
	Processed    bool   `db:"processed"`
}

type DeliveryQueueRow struct {
	ID            int64   `db:"id"`
	TargetNode    string  `db:"target_node"`
	MessageID     string  `db:"message_id"`
	MessageJSON   string  `db:"message_json"`
	NextAttemptAt string  `db:"next_attempt_at"`
	AttemptCount  int     `db:"attempt_count"`
	CreatedAt     string  `db:"created_at"`
	LastError     *string `db:"last_error"`
}
