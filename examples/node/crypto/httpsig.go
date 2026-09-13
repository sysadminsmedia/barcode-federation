package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	maxClockSkew = 5 * time.Minute
	sigLabel     = "sig1"
)

// SignRequest signs an HTTP request per RFC 9421 covering the required FBS components.
// Covers: @method, @target-uri, @authority, date, and content-digest (for POST/PUT).
func SignRequest(req *http.Request, keyID string, privKey ed25519.PrivateKey) error {
	now := time.Now().UTC()
	req.Header.Set("Date", now.Format(http.TimeFormat))

	if req.Body != nil && (req.Method == "POST" || req.Method == "PUT") {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		req.Body = io.NopCloser(strings.NewReader(string(body)))
		h := sha256.Sum256(body)
		digest := "sha-256=:" + base64.StdEncoding.EncodeToString(h[:]) + ":"
		req.Header.Set("Content-Digest", digest)
	}

	components := []string{"@method", "@target-uri", "@authority", "date"}
	if req.Header.Get("Content-Digest") != "" {
		components = append(components, "content-digest")
	}

	sigBase := buildSignatureBase(req, components, now)

	sig := ed25519.Sign(privKey, []byte(sigBase))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	inputParts := make([]string, len(components))
	for i, c := range components {
		if strings.HasPrefix(c, "@") {
			inputParts[i] = `"` + c + `"`
		} else {
			inputParts[i] = `"` + c + `"`
		}
	}
	created := now.Unix()
	sigInput := fmt.Sprintf("%s=(%s);created=%d;keyid=\"%s\";alg=\"ed25519\"",
		sigLabel, strings.Join(inputParts, " "), created, keyID)

	req.Header.Set("Signature-Input", sigInput)
	req.Header.Set("Signature", fmt.Sprintf("%s=:%s:", sigLabel, sigB64))
	return nil
}

// VerifyRequest verifies HTTP signature on an inbound request.
// Returns the keyID from the signature and nil error on success.
func VerifyRequest(req *http.Request, keyLookup func(keyID string) (ed25519.PublicKey, error)) (string, error) {
	sigInput := req.Header.Get("Signature-Input")
	sigHeader := req.Header.Get("Signature")
	if sigInput == "" || sigHeader == "" {
		return "", fmt.Errorf("missing Signature-Input or Signature header")
	}

	keyID, created, components, err := parseSignatureInput(sigInput)
	if err != nil {
		return "", fmt.Errorf("parse signature input: %w", err)
	}

	createdTime := time.Unix(created, 0)
	if math.Abs(time.Since(createdTime).Seconds()) > maxClockSkew.Seconds() {
		return "", fmt.Errorf("signature timestamp outside allowed skew")
	}

	dateStr := req.Header.Get("Date")
	if dateStr != "" {
		dateTime, err := http.ParseTime(dateStr)
		if err == nil {
			if math.Abs(time.Since(dateTime).Seconds()) > maxClockSkew.Seconds() {
				return "", fmt.Errorf("date header outside allowed skew")
			}
		}
	}

	pubKey, err := keyLookup(keyID)
	if err != nil {
		return "", fmt.Errorf("key lookup for %s: %w", keyID, err)
	}

	sigBase := buildSignatureBase(req, components, createdTime)

	sigB64, err := extractSignatureValue(sigHeader)
	if err != nil {
		return "", err
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}

	if !ed25519.Verify(pubKey, []byte(sigBase), sigBytes) {
		return "", fmt.Errorf("signature verification failed")
	}
	return keyID, nil
}

func buildSignatureBase(req *http.Request, components []string, created time.Time) string {
	var lines []string
	for _, c := range components {
		var val string
		switch c {
		case "@method":
			val = req.Method
		case "@target-uri":
			scheme := "https"
			if req.TLS == nil && req.Header.Get("X-Forwarded-Proto") == "" {
				scheme = "http"
			}
			val = scheme + "://" + req.Host + req.URL.RequestURI()
		case "@authority":
			val = req.Host
		default:
			val = req.Header.Get(http.CanonicalHeaderKey(c))
		}
		lines = append(lines, fmt.Sprintf(`"%s": %s`, c, val))
	}

	inputParts := make([]string, len(components))
	for i, c := range components {
		inputParts[i] = `"` + c + `"`
	}
	params := fmt.Sprintf("(%s);created=%d;keyid=\"\";alg=\"ed25519\"",
		strings.Join(inputParts, " "), created.Unix())
	lines = append(lines, fmt.Sprintf(`"@signature-params": %s`, params))
	return strings.Join(lines, "\n")
}

func parseSignatureInput(input string) (keyID string, created int64, components []string, err error) {
	// Format: sig1=("@method" "@target-uri" ...);created=123;keyid="...";alg="ed25519"
	eqIdx := strings.Index(input, "=(")
	if eqIdx < 0 {
		return "", 0, nil, fmt.Errorf("invalid signature input format")
	}
	rest := input[eqIdx+1:]

	parenClose := strings.Index(rest, ")")
	if parenClose < 0 {
		return "", 0, nil, fmt.Errorf("missing closing paren")
	}
	compStr := rest[1:parenClose]

	for _, part := range strings.Split(compStr, " ") {
		part = strings.Trim(part, `"`)
		if part != "" {
			components = append(components, part)
		}
	}

	params := rest[parenClose+1:]
	for _, param := range strings.Split(params, ";") {
		param = strings.TrimSpace(param)
		if strings.HasPrefix(param, "created=") {
			fmt.Sscanf(param, "created=%d", &created)
		} else if strings.HasPrefix(param, "keyid=") {
			keyID = strings.Trim(strings.TrimPrefix(param, "keyid="), `"`)
		}
	}
	return keyID, created, components, nil
}

func extractSignatureValue(header string) (string, error) {
	// Format: sig1=:base64value:
	eqIdx := strings.Index(header, "=:")
	if eqIdx < 0 {
		return "", fmt.Errorf("invalid signature header format")
	}
	rest := header[eqIdx+2:]
	endIdx := strings.Index(rest, ":")
	if endIdx < 0 {
		return "", fmt.Errorf("missing trailing colon in signature")
	}
	return rest[:endIdx], nil
}
