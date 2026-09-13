package model

import "encoding/json"

type BarcodeRecord struct {
	FBS                  string              `json:"fbs"`
	Type                 string              `json:"type"`
	ID                   string              `json:"id"`
	CBI                  string              `json:"cbi"`
	Symbology            string              `json:"symbology"`
	Value                string              `json:"value"`
	Status               string              `json:"status"`
	SubmittedBy          string              `json:"submittedBy"`
	SubmittedAt          string              `json:"submittedAt"`
	UpdatedAt            string              `json:"updatedAt"`
	ExpiresAt            string              `json:"expiresAt,omitempty"`
	OriginNode           string              `json:"originNode"`
	NamespaceClaim       *NamespaceClaimRef  `json:"namespaceClaim,omitempty"`
	MetadataSchema       string              `json:"metadataSchema"`
	MetadataSchemaDigest string              `json:"metadataSchemaDigest,omitempty"`
	Metadata             json.RawMessage     `json:"metadata"`
	GS1CrossRef          *GS1CrossRef        `json:"gs1CrossRef,omitempty"`
	LifecycleSchema      string              `json:"lifecycleSchema,omitempty"`
	CustodyChain         []CustodyEntry      `json:"custodyChain,omitempty"`
	CurrentCustodian     string              `json:"currentCustodian,omitempty"`
	RelatedRecords       []RelatedRecord     `json:"relatedRecords,omitempty"`
	RevisionHistory      []RevisionEntry     `json:"revisionHistory"`
	Signature            string              `json:"signature"`
}

// CustodyEntry records a single custody transfer in the chain.
type CustodyEntry struct {
	From            string  `json:"from"`
	To              string  `json:"to"`
	At              string  `json:"at"`
	Condition       string  `json:"condition"` // accepted, damaged, partial, rejected
	FromSignature   string  `json:"fromSignature"`
	ToSignature     string  `json:"toSignature,omitempty"`
	DelegateExpiry  string  `json:"delegateExpiry,omitempty"`
}

// RelatedRecord links this record to another record with a typed relationship.
type RelatedRecord struct {
	CBI          string `json:"cbi"`
	RecordID     string `json:"recordId,omitempty"`
	Relationship string `json:"relationship"` // contains, contained-in, replaces, replaced-by, fulfills, derived-from, cross-reference
	Quantity     int    `json:"quantity,omitempty"`
	Description  string `json:"description,omitempty"`
}

// CustodyTransferNotice is the object payload for a CustodyTransfer federation activity.
type CustodyTransferNotice struct {
	Type                 string   `json:"type"`
	RecordID             string   `json:"recordId"`
	RecordCBI            string   `json:"recordCbi"`
	FromActor            string   `json:"fromActor"`
	ToActor              string   `json:"toActor"`
	TransferredAt        string   `json:"transferredAt"`
	Condition            string   `json:"condition"`
	DelegateCapabilities []string `json:"delegateCapabilities"`
	ExpiresAt            string   `json:"expiresAt,omitempty"`
	FromSignature        string   `json:"fromSignature"`
	ToSignature          string   `json:"toSignature,omitempty"`
}

// CustodyAcceptance is the object payload for a CustodyAccept federation activity.
type CustodyAcceptance struct {
	Type              string `json:"type"`
	TransferNoticeID  string `json:"transferNoticeId"`
	RecordID          string `json:"recordId"`
	AcceptedBy        string `json:"acceptedBy"`
	AcceptedAt        string `json:"acceptedAt"`
	Condition         string `json:"condition"`
	ToSignature       string `json:"toSignature"`
}

type NamespaceClaimRef struct {
	ClaimID        string `json:"claimId"`
	ClaimedBy      string `json:"claimedBy"`
	ClaimSignature string `json:"claimSignature"`
}

type GS1CrossRef struct {
	CBI               string `json:"cbi"`
	CrossRefSignature string `json:"crossRefSignature"`
}

type RevisionEntry struct {
	RevisionID                 string `json:"revisionId"`
	At                         string `json:"at"`
	By                         string `json:"by"`
	ChangeType                 string `json:"changeType"`
	Summary                    string `json:"summary,omitempty"`
	PreviousRevisionSignature  *string `json:"previousRevisionSignature"`
}

// RecordRow is the database row representation.
type RecordRow struct {
	ID                   string  `db:"id"`
	CBI                  string  `db:"cbi"`
	Symbology            string  `db:"symbology"`
	Value                string  `db:"value"`
	Status               string  `db:"status"`
	SubmittedBy          string  `db:"submitted_by"`
	SubmittedAt          string  `db:"submitted_at"`
	UpdatedAt            string  `db:"updated_at"`
	ExpiresAt            *string `db:"expires_at"`
	OriginNode           string  `db:"origin_node"`
	NamespaceClaim       *string `db:"namespace_claim"`
	MetadataSchema       string  `db:"metadata_schema"`
	MetadataSchemaDigest *string `db:"metadata_schema_digest"`
	Metadata             string  `db:"metadata"`
	GS1CrossRef          *string `db:"gs1_cross_ref"`
	LifecycleSchema      *string `db:"lifecycle_schema"`
	Signature            string  `db:"signature"`
	RawJSON              string  `db:"raw_json"`
	VerificationLevel    int     `db:"verification_level"`
}

// RecordSubmission is what the client sends to POST /api/v1/records.
type RecordSubmission struct {
	Symbology            string              `json:"symbology"`
	Value                string              `json:"value"`
	ExpiresAt            string              `json:"expiresAt,omitempty"`
	NamespaceClaim       *NamespaceClaimRef  `json:"namespaceClaim,omitempty"`
	MetadataSchema       string              `json:"metadataSchema"`
	MetadataSchemaDigest string              `json:"metadataSchemaDigest,omitempty"`
	Metadata             json.RawMessage     `json:"metadata"`
	GS1CrossRef          *GS1CrossRef        `json:"gs1CrossRef,omitempty"`
	LifecycleSchema      string              `json:"lifecycleSchema,omitempty"`
	RelatedRecords       []RelatedRecord     `json:"relatedRecords,omitempty"`
}

var ValidSymbologies = map[string]bool{
	"upc-a": true, "upc-e": true, "ean-13": true, "ean-8": true,
	"code-39": true, "code-128": true, "itf-14": true, "gs1-128": true,
	"qr-code": true, "data-matrix": true, "pdf-417": true, "aztec": true,
	"gs1-databar": true,
}
