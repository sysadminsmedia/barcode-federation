package model

type VerificationSession struct {
	ID              string  `json:"id" db:"id"`
	ActorID         string  `json:"actorId" db:"actor_id"`
	Status          string  `json:"status" db:"status"`
	ClaimedPrefixes string  `json:"claimedPrefixes" db:"claimed_prefixes"`
	EvidencePackage *string `json:"evidencePackage,omitempty" db:"evidence_package"`
	CreatedAt       string  `json:"createdAt" db:"created_at"`
	UpdatedAt       string  `json:"updatedAt" db:"updated_at"`
	Result          *string `json:"result,omitempty" db:"result"`
}

type VerificationRequest struct {
	ActorID         string      `json:"actorId"`
	ClaimedPrefixes []string    `json:"claimedPrefixes"`
	EvidencePackage interface{} `json:"evidencePackage"`
}
