package model

type KeySuccessionDeclaration struct {
	FBS                    string    `json:"fbs"`
	Type                   string    `json:"type"`
	ID                     string    `json:"id"`
	OldFNI                 string    `json:"oldFni"`
	OldPublicKey           PublicKey `json:"oldPublicKey"`
	SuccessorFNI           string    `json:"successorFni"`
	SuccessorPublicKey     PublicKey `json:"successorPublicKey"`
	Scope                  string    `json:"scope"`
	DeclaredAt             string    `json:"declaredAt"`
	ExpiresAt              string    `json:"expiresAt"`
	OldKeySignature        string    `json:"oldKeySignature"`
	SuccessorKeySignature  string    `json:"successorKeySignature"`
}

type PublicKey struct {
	Algorithm    string `json:"algorithm"`
	PublicKeyPem string `json:"publicKeyPem"`
}

type SuccessionRow struct {
	ID            string `db:"id"`
	OldFNI        string `db:"old_fni"`
	SuccessorFNI  string `db:"successor_fni"`
	DeclaredAt    string `db:"declared_at"`
	ExpiresAt     string `db:"expires_at"`
	DocumentJSON  string `db:"document_json"`
}
