package model

import "encoding/json"

type NamespaceClaim struct {
	FBS           string          `json:"fbs"`
	Type          string          `json:"type"`
	ID            string          `json:"id"`
	ClaimedBy     string          `json:"claimedBy"`
	NamespaceType string          `json:"namespaceType"`
	Scope         json.RawMessage `json:"scope"`
	IssuedAt      string          `json:"issuedAt"`
	ExpiresAt     string          `json:"expiresAt,omitempty"`
	Evidence      json.RawMessage `json:"evidence"`
	Signature     string          `json:"signature"`
}

type GS1Scope struct {
	Symbologies []string `json:"symbologies"`
	Prefixes    []string `json:"prefixes"`
}

type SelfScope struct {
	FNI string `json:"fni"`
}

type SelfSovereignEvidence struct {
	Type              string          `json:"type"`
	NamespacePublicKey json.RawMessage `json:"namespacePublicKey"`
	DerivedFNI        string          `json:"derivedFni"`
}

type ClaimRow struct {
	ID            string  `db:"id"`
	ClaimedBy     string  `db:"claimed_by"`
	NamespaceType string  `db:"namespace_type"`
	Scope         string  `db:"scope"`
	IssuedAt      string  `db:"issued_at"`
	ExpiresAt     *string `db:"expires_at"`
	Evidence      string  `db:"evidence"`
	Signature     string  `db:"signature"`
	RawJSON       string  `db:"raw_json"`
}

type ClaimSubmission struct {
	NamespaceType string          `json:"namespaceType"`
	Scope         json.RawMessage `json:"scope"`
	ExpiresAt     string          `json:"expiresAt,omitempty"`
	Evidence      json.RawMessage `json:"evidence"`
}
