package model

import "encoding/json"

type Actor struct {
	ID              string   `db:"id" json:"id"`
	Profile         string   `db:"profile" json:"profile,omitempty"`
	DisplayName     string   `db:"display_name" json:"displayName,omitempty"`
	Node            string   `db:"node" json:"node"`
	PublicKeyPem    string   `db:"public_key_pem" json:"-"`
	CapabilitiesRaw string   `db:"capabilities" json:"-"`
	NamespaceClaimsRaw string `db:"namespace_claims" json:"-"`
	APIToken        *string  `db:"api_token" json:"-"`
	PrivateKeyPem   *string  `db:"private_key_pem" json:"-"`
	Capabilities    []string `db:"-" json:"-"`
}

func (a *Actor) ParseCapabilities() {
	if a.CapabilitiesRaw != "" {
		json.Unmarshal([]byte(a.CapabilitiesRaw), &a.Capabilities)
	}
}

func (a *Actor) HasCapability(cap string) bool {
	a.ParseCapabilities()
	for _, c := range a.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

type ActorProfile struct {
	FBS             string          `json:"fbs"`
	Type            string          `json:"type"`
	ID              string          `json:"id"`
	Profile         string          `json:"profile,omitempty"`
	DisplayName     string          `json:"displayName,omitempty"`
	Node            string          `json:"node"`
	Outbox          string          `json:"outbox,omitempty"`
	PublicKey       ActorPublicKey  `json:"publicKey"`
	Capabilities    []string        `json:"capabilities"`
	NamespaceClaims []string        `json:"namespaceClaims,omitempty"`
}

type ActorPublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	Algorithm    string `json:"algorithm"`
	PublicKeyPem string `json:"publicKeyPem"`
}

func (a *Actor) ToProfile(nodeBaseURL string) ActorProfile {
	a.ParseCapabilities()
	var claims []string
	if a.NamespaceClaimsRaw != "" && a.NamespaceClaimsRaw != "[]" {
		json.Unmarshal([]byte(a.NamespaceClaimsRaw), &claims)
	}

	return ActorProfile{
		FBS:         "1.0",
		Type:        "Actor",
		ID:          a.ID,
		Profile:     a.Profile,
		DisplayName: a.DisplayName,
		Node:        nodeBaseURL,
		Outbox:      nodeBaseURL + "/actors/" + extractLocalID(a.ID) + "/outbox",
		PublicKey: ActorPublicKey{
			ID:           a.ID + "#main-key",
			Owner:        a.ID,
			Algorithm:    "Ed25519",
			PublicKeyPem: a.PublicKeyPem,
		},
		Capabilities:    a.Capabilities,
		NamespaceClaims: claims,
	}
}

func extractLocalID(aid string) string {
	// fbs://host/localid -> localid
	if len(aid) < 7 {
		return aid
	}
	rest := aid[6:] // skip "fbs://"
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			return rest[i+1:]
		}
	}
	return rest
}
