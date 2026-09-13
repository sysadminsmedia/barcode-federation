package model

type NodeMeta struct {
	FBS              string      `json:"fbs"`
	Type             string      `json:"type"`
	NodeID           string      `json:"nodeId"`
	DisplayName      string      `json:"displayName"`
	OperatedBy       string      `json:"operatedBy,omitempty"`
	Version          string      `json:"version,omitempty"`
	Capabilities     []string    `json:"capabilities,omitempty"`
	FederationPolicy string      `json:"federationPolicy,omitempty"`
	Inbox            string      `json:"inbox"`
	Outbox           string      `json:"outbox,omitempty"`
	Peers            string      `json:"peers,omitempty"`
	PublicKey        NodeKey     `json:"publicKey"`
	TrustAnchors     []string    `json:"trustAnchors,omitempty"`
	SchemaRegistries []string    `json:"schemaRegistries,omitempty"`
	Contact          string      `json:"contact,omitempty"`
	TermsOfService   string      `json:"termsOfService,omitempty"`
}

type NodeKey struct {
	ID           string `json:"id"`
	Algorithm    string `json:"algorithm"`
	PublicKeyPem string `json:"publicKeyPem"`
}

type PeerEntry struct {
	NodeID       string `json:"nodeId"`
	Meta         string `json:"meta"`
	AddedAt      string `json:"addedAt"`
	Relationship string `json:"relationship"`
}

type PeerList struct {
	FBS         string      `json:"fbs"`
	Type        string      `json:"type"`
	NodeID      string      `json:"nodeId"`
	PublishedAt string      `json:"publishedAt"`
	TTL         int         `json:"ttl"`
	Peers       []PeerEntry `json:"peers"`
}

type Peer struct {
	NodeID           string  `db:"node_id"`
	MetaURL          string  `db:"meta_url"`
	Relationship     string  `db:"relationship"`
	AddedAt          string  `db:"added_at"`
	LastSeen         *string `db:"last_seen"`
	FederationPolicy *string `db:"federation_policy"`
	PublicKeyPem     *string `db:"public_key_pem"`
}
