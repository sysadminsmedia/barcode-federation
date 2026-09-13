# FBS Reference Node

A reference implementation of an FBS (Federated Barcode System) node, implementing the protocol defined in [RFC FBS0001](../../rfc/FBS0001.md).

This is a single-binary Go server with SQLite storage, intended for testing, development, and as a starting point for production implementations.

## Features

- Full FBS 1.0 protocol API (barcode lookup, record submission, bulk import, subscriptions)
- Federation inbox/outbox with HTTP Message Signatures (RFC 9421)
- Capability-based access control with built-in profiles (PublicUser, RetailOperator, Manufacturer)
- Namespace claims for both GS1-track and Self-Sovereign (SSN) barcodes
- FNI derivation with normative test vector verification
- Record signing with Ed25519 and canonical JSON serialization
- Conflict resolution and record ranking per Section 4.9
- Key Succession Declaration handling
- Background workers for delivery retry, record expiry, webhook delivery, and bulk import processing
- FROST threshold signatures (RFC 9591) for Level 3 Verifier Quorum badges
- Verification Transparency Log with Merkle tree, SCT issuance, and inclusion proofs

## Quick Start

```bash
# Build
go build -o fbs-node .

# Initialize (generates config.toml and node-key.pem)
./fbs-node -init

# Edit config.toml as needed, then start the node:
./fbs-node -config config.toml

# In a separate terminal, create actors:
./fbs-node -config config.toml -create-actor acme-corp -profile Manufacturer -display-name "Acme Corporation"
./fbs-node -config config.toml -create-actor retailer -profile RetailOperator -display-name "Big Retail"
```

Save the API tokens printed for each actor. You will need them for authenticated requests.

## API Endpoints

### Unauthenticated

| Endpoint | Method | Description |
|---|---|---|
| `/.well-known/fbs/meta` | GET | Node metadata |
| `/.well-known/fbs/actors/{id}` | GET | Actor profile |
| `/.well-known/fbs/webrecord?actor={id}` | GET | Actor discovery |
| `/.well-known/fbs/peers` | GET | Peer list |
| `/api/v1/barcodes?q=...&symbology=...` | GET | Search barcodes |
| `/api/v1/barcodes/{symbology}/{value}` | GET | Lookup by CBI |
| `/api/v1/records/{id}` | GET | Fetch record |
| `/api/v1/actors/{id}/claims` | GET | List actor claims |
| `/api/v1/successions?since=...` | GET | List succession docs |

### Authenticated (Bearer token)

| Endpoint | Method | Capability | Description |
|---|---|---|---|
| `/api/v1/records` | POST | record:submit | Submit record |
| `/api/v1/records/{id}` | PUT | record:update:own | Update record |
| `/api/v1/bulk-import` | POST | record:submit:bulk | Bulk import |
| `/api/v1/bulk-import/{id}` | GET | (any) | Poll bulk job |
| `/api/v1/claims` | POST | namespace:claim | Submit claim |
| `/api/v1/subscriptions` | POST | subscription:create | Create webhook |
| `/api/v1/verification/quorum` | POST | (any) | Start L3 verification |
| `/api/v1/verification/quorum/{id}` | GET | (any) | Poll verification |

### Federation

| Endpoint | Method | Description |
|---|---|---|
| `/federation/inbox` | POST | Receive federation messages |
| `/federation/outbox` | GET | Paginated outbox log |

### Transparency Log

| Endpoint | Method | Description |
|---|---|---|
| `/fbs-log/v1/metadata` | GET | Log public key, MMD, tree size |
| `/fbs-log/v1/add-badge` | POST | Submit a badge, receive an SCT |
| `/fbs-log/v1/get-entries?start=N&end=M` | GET | Retrieve entries by range |
| `/fbs-log/v1/get-proof-by-hash?hash=...` | GET | Merkle inclusion proof |
| `/fbs-log/v1/get-sth` | GET | Current Signed Tree Head |

## Running Tests

```bash
# Run all tests (36 tests across 4 packages)
go test ./... -v

# Run only the integration tests (10 tests, full HTTP round-trips)
go test -v -run TestIntegration .

# Run only the FROST threshold signature tests
go test -v -run FROST ./crypto/

# Run only the transparency log tests
go test -v ./transparencylog/

# Run only the quorum ceremony tests
go test -v ./quorum/
```

The integration tests spin up a real HTTP server per test with a temporary SQLite database and exercise every endpoint, including the full Level 3 quorum ceremony (FROST signing + transparency log submission).

## Testing with curl

```bash
TOKEN="fbs_acme-corp_..."  # from the create-actor output

# Node metadata
curl http://localhost:8080/.well-known/fbs/meta | jq .

# Submit a barcode record
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbology": "ean-13",
    "value": "5901234123457",
    "metadataSchema": "https://schemas.fbs-community.example/metadata/food/v1.0.0.json",
    "metadata": {
      "name": "Acme Organic Oats 500g",
      "languages": ["en"],
      "brand": "Acme",
      "ingredients": ["Whole Grain Rolled Oats"]
    }
  }' | jq .

# Search for it
curl "http://localhost:8080/api/v1/barcodes?q=5901234" | jq .

# Lookup by CBI
curl http://localhost:8080/api/v1/barcodes/ean-13/5901234123457 | jq .

# Submit a namespace claim
curl -s -X POST http://localhost:8080/api/v1/claims \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "namespaceType": "gs1",
    "scope": {"symbologies": ["ean-13"], "prefixes": ["5901234"]},
    "expiresAt": "2028-01-15T00:00:00Z",
    "evidence": {"type": "GS1CompanyPrefix", "verifiedPrefixes": ["5901234"]}
  }' | jq .

# Start a Level 3 quorum verification
curl -s -X POST http://localhost:8080/api/v1/verification/quorum \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "claimedPrefixes": ["5901234"],
    "evidencePackage": {
      "gs1PrefixLicense": "LICENSE-12345",
      "businessRegistration": "REG-67890",
      "verifiedDomain": "acme.example.com"
    }
  }' | jq .
# Copy the sessionId from the response

# Poll until status is "completed" — the badge will contain
# a FROST threshold signature and transparency log SCT
curl -s "http://localhost:8080/api/v1/verification/quorum/SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Inspect the transparency log
curl http://localhost:8080/fbs-log/v1/metadata | jq .
curl http://localhost:8080/fbs-log/v1/get-sth | jq .
curl "http://localhost:8080/fbs-log/v1/get-entries?start=0&end=10" | jq .
```

## Self-Sovereign Namespace (SSN) Walkthrough

Self-Sovereign Namespaces allow any actor to create globally unique barcodes without GS1 membership. Uniqueness is guaranteed by Ed25519 cryptography — the namespace identifier (FNI) is derived from a public key, and ownership is proven by signing with the corresponding private key.

This walkthrough covers the complete SSN flow: deriving an FNI, claiming a namespace, submitting barcodes under it, and looking them up.

### Concepts

| Term | Description |
|---|---|
| **FNI** | FBS Namespace Identifier — a 36-character string like `fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7` derived from an Ed25519 public key |
| **FBS URI** | The barcode payload: `fbs://{fni}/{local-id}` — globally unique per item |
| **Local ID** | Actor-chosen identifier for each product/shipment within the namespace |
| **Namespace Claim** | A signed assertion binding an FNI to an actor, verified cryptographically |

The FNI is like a GS1 company prefix — it identifies *who* is issuing barcodes. The local-id identifies *what*. The complete FBS URI is what gets encoded into a QR code or Aztec symbol.

### 1. Create a manufacturer actor

```bash
./fbs-node -config config.toml \
  -create-actor dhl-express -profile Manufacturer \
  -display-name "DHL Express" \
  -capabilities '["record:read","record:submit","record:submit:bulk","record:update:own","record:retract:own","namespace:claim","subscription:create"]'
# Save the printed token as SSN_TOKEN
```

### 2. Get the actor's public key

The actor's public key is in their profile. You need it to derive the FNI.

```bash
curl -s http://localhost:8080/.well-known/fbs/actors/dhl-express | jq .publicKey.publicKeyPem
```

### 3. Derive the FNI

The FNI derivation algorithm (RFC FBS0001 Section 4.2.5.2):

1. SHA-256 hash the raw 32-byte Ed25519 public key
2. Truncate to first 20 bytes
3. Base32 encode (RFC 4648 §6), lowercase, no padding
4. Prepend `fni1`

Using OpenSSL and standard tools to derive it manually:

```bash
# Extract raw public key bytes from the PEM, compute FNI
# (In practice, your application code calls crypto.DeriveFNI())

# The Go implementation provides this in crypto/fni.go:
#   fni, _ := crypto.DeriveFNI(publicKeyBytes)
#   uri := "fbs://" + fni + "/SHIPMENT-001"
```

The normative test vector from the RFC:

```
Public key (hex): 4f6f787a4211deadbeefcafebabe000102030405060708090a0b0c0d0e0f1011
Derived FNI:      fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7
Example URI:      fbs://fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7/SHIPMENT-001
```

### 4. Submit a self-sovereign namespace claim

This registers the actor's authority over the FNI. The node cryptographically verifies that the FNI is correctly derived from the provided public key before accepting the claim.

```bash
SSN_TOKEN="fbs_dhl-express_..."  # from step 1

curl -s -X POST http://localhost:8080/api/v1/claims \
  -H "Authorization: Bearer $SSN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "namespaceType": "self",
    "scope": {
      "fni": "fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7"
    },
    "evidence": {
      "type": "SelfSovereign",
      "namespacePublicKey": {
        "algorithm": "Ed25519",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEAT294ekIR3q2+78r+ur4AAQIDBAUGB...\n-----END PUBLIC KEY-----\n"
      },
      "derivedFni": "fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7"
    }
  }' | jq .
```

The node validates:
- `scope.fni` is syntactically valid (36 chars, `fni1` prefix, base32 alphabet)
- `evidence.namespacePublicKey` is a valid Ed25519 key
- Deriving the FNI from the public key produces a value matching `scope.fni`
- `evidence.derivedFni` matches as well (redundant but required by the RFC for error detection)

If the FNI doesn't match the key, the request is rejected with a 400 error.

### 5. Submit a barcode record under the SSN

With the namespace claimed, submit records using FBS URIs as the barcode value. The symbology is whatever 2D format you encode the URI into (QR, Aztec, Data Matrix).

```bash
# A shipment tracking barcode
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "Authorization: Bearer $SSN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbology": "qr-code",
    "value": "fbs://fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7/SHIPMENT-2026-0042",
    "metadataSchema": "https://schemas.fbs-community.example/metadata/electronics/v1.0.0.json",
    "metadata": {
      "name": "DHL Shipment TRK-2026-0042",
      "languages": ["en"],
      "brand": "DHL Express",
      "manufacturer": "Various",
      "modelNumber": "N/A"
    },
    "expiresAt": "2026-05-01T00:00:00Z",
    "namespaceClaim": {
      "claimId": "https://localhost:8080/api/v1/actors/dhl-express/claims/CLAIM_ID_HERE",
      "claimedBy": "fbs://localhost:8080/dhl-express",
      "claimSignature": "base64url-signature-here"
    }
  }' | jq .
```

The CBI (Canonical Barcode Identifier) is automatically constructed as `qr-code:fbs://fni1.../SHIPMENT-2026-0042`. Because the value starts with `fbs://`, nodes know this is a self-sovereign barcode.

You can also submit without the `namespaceClaim` field — the record is still valid, it just won't carry the namespace ownership proof. The claim can be referenced later.

### 6. Submit more barcodes under the same namespace

The FNI is the same for all barcodes from this keypair. Only the local-id changes:

```bash
# A product barcode
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "Authorization: Bearer $SSN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbology": "aztec",
    "value": "fbs://fni1ng3am5odtfy3gah6gvz7ial24kgiqpe7/PRODUCT-A001",
    "metadataSchema": "https://schemas.fbs-community.example/metadata/electronics/v1.0.0.json",
    "metadata": {
      "name": "Wireless Bluetooth Speaker",
      "languages": ["en"],
      "brand": "SoundCo",
      "manufacturer": "SoundCo Ltd",
      "modelNumber": "BT-500X"
    }
  }' | jq .
```

### 7. Look up SSN barcodes

Any FBS node can resolve self-sovereign barcodes. The value in the URL must be URL-encoded since it contains `://` and `/`:

```bash
# Lookup by CBI (URL-encode the FBS URI)
curl -s "http://localhost:8080/api/v1/barcodes/qr-code/fbs%3A%2F%2Ffni1ng3am5odtfy3gah6gvz7ial24kgiqpe7%2FSHIPMENT-2026-0042" | jq .

# Search by partial value
curl -s "http://localhost:8080/api/v1/barcodes?q=SHIPMENT-2026" | jq .

# Filter to only SSN records (values starting with fbs://)
curl -s "http://localhost:8080/api/v1/barcodes?q=fbs://" | jq .
```

### 8. Verify the claim

List claims for the actor to see the namespace claim:

```bash
curl -s http://localhost:8080/api/v1/actors/dhl-express/claims | jq .
```

The response includes the full claim with scope, evidence, and signature. Any node receiving this claim via federation can independently verify it by deriving the FNI from the evidence public key — no external registry lookup required.

### 9. Record expiry (time-bounded SSN records)

SSN records with `expiresAt` (like the shipment above) automatically transition to `"expired"` status when the timestamp is reached. No signature or federation activity is needed — the node handles it autonomously. This is the recommended approach for tracking numbers, event tickets, and other short-lived identifiers.

```bash
# After expiry, the record no longer appears in default searches
curl -s "http://localhost:8080/api/v1/barcodes?q=SHIPMENT-2026" | jq .total
# Returns 0

# But it's still retrievable with the status filter for audit purposes
curl -s "http://localhost:8080/api/v1/barcodes?q=SHIPMENT-2026&status=expired" | jq .
```

### SSN vs GS1: When to use which

The two tracks are not mutually exclusive. An organization can use both simultaneously — for example, keeping GS1 barcodes on retail products for supply chain compatibility while using SSN for internal tracking, logistics, or new product lines. The `gs1CrossRef` field on SSN records (Section 4.2.5.6) allows explicit linking between the two, making SSN a natural transition path for organizations exploring decentralized barcode infrastructure without abandoning their existing GS1 investment.

| Scenario | Track | Why |
|---|---|---|
| Consumer goods with existing GS1 barcodes | GS1 | Already in supply chain scanners |
| Shipment tracking numbers | SSN | No GS1 membership needed; use `expiresAt` |
| Small craft producer | SSN | No fees; start issuing immediately |
| Library system | SSN | Internal namespace; self-sovereign |
| Event tickets | SSN | Time-bounded; `expiresAt` handles lifecycle |
| Major CPG manufacturer | GS1 | Existing infrastructure; Level 3 verification available |
| Manufacturer with GS1 adding internal tracking | Both | GS1 for retail, SSN for warehouse/logistics |
| Organization transitioning to decentralized IDs | Both | SSN for new products, GS1 for legacy; cross-reference with `gs1CrossRef` |

## Two-Node Federation Test

This walks through running two FBS nodes that federate with each other. A record submitted on node A will automatically propagate to node B via the federation delivery worker.

### 1. Build the binary

```bash
cd examples/node
go build -o fbs-node .
```

### 2. Initialize both nodes

```bash
# Node A
mkdir -p /tmp/fbs-node-a && cd /tmp/fbs-node-a
/path/to/fbs-node -init
```

Edit `/tmp/fbs-node-a/config.toml`:

```toml
[node]
host = "localhost:8080"
display_name = "Node A"
operated_by = "Operator A"
key_file = "node-key.pem"

[database]
path = "fbs-node.db"

[server]
addr = ":8080"

[federation]
policy = "open"
peers = ["http://localhost:9090"]
```

```bash
# Node B
mkdir -p /tmp/fbs-node-b && cd /tmp/fbs-node-b
/path/to/fbs-node -init
```

Edit `/tmp/fbs-node-b/config.toml`:

```toml
[node]
host = "localhost:9090"
display_name = "Node B"
operated_by = "Operator B"
key_file = "node-key.pem"

[database]
path = "fbs-node.db"

[server]
addr = ":9090"

[federation]
policy = "open"
peers = ["http://localhost:8080"]
```

### 3. Create actors on node A

```bash
cd /tmp/fbs-node-a
/path/to/fbs-node -config config.toml \
  -create-actor acme-corp -profile Manufacturer -display-name "Acme Corp"
# Save the printed token as TOKEN_A
```

### 4. Start both nodes

```bash
# Terminal 1
cd /tmp/fbs-node-a && /path/to/fbs-node -config config.toml

# Terminal 2
cd /tmp/fbs-node-b && /path/to/fbs-node -config config.toml
```

### 5. Submit a record on node A

```bash
curl -s -X POST http://localhost:8080/api/v1/records \
  -H "Authorization: Bearer $TOKEN_A" \
  -H "Content-Type: application/json" \
  -d '{
    "symbology": "ean-13",
    "value": "5901234123457",
    "metadataSchema": "https://schemas.fbs-community.example/metadata/food/v1.0.0.json",
    "metadata": {
      "name": "Acme Organic Oats 500g",
      "languages": ["en"],
      "brand": "Acme",
      "ingredients": ["Whole Grain Rolled Oats"]
    }
  }' | jq .
```

### 6. Verify it federated to node B

The delivery worker runs every 10 seconds. Wait a moment, then query node B:

```bash
# Search on node B (port 9090)
curl "http://localhost:9090/api/v1/barcodes?q=5901234" | jq .

# Or lookup the exact CBI
curl http://localhost:9090/api/v1/barcodes/ean-13/5901234123457 | jq .
```

You should see the record with `originNode` pointing to node A, confirming it arrived via federation.

### 7. Check federation state

```bash
# Node A's outbox (should show the Publish message)
curl http://localhost:8080/federation/outbox | jq .totalItems

# Node B's peer list (should show node A)
curl http://localhost:9090/.well-known/fbs/peers | jq .
```

## What Is Stubbed

- **External evidence verification** — The quorum verifier accepts evidence structurally but does not make real API calls to GS1 GEPIR, Companies House, etc. Production implementations would replace `verifier.go`'s `verifyAgainstSource()` with real external calls.
- **DNS TXT challenge** — Level 1 verification not implemented.
- **TLS** — The node serves HTTP only; expects a reverse proxy (nginx, Caddy) for TLS termination in production.
- **OAuth 2.0** — Uses simple Bearer API tokens instead of full OAuth flows.
- **Inbound HTTP Signature verification** — Federation inbox accepts messages with a warning log rather than cryptographically verifying the sender's HTTP signature. Outbound signatures are fully implemented.

## Dependencies

| Package | Purpose |
|---|---|
| github.com/go-chi/chi/v5 | HTTP routing |
| modernc.org/sqlite | Pure-Go SQLite (no CGO) |
| github.com/jmoiron/sqlx | SQL convenience |
| github.com/BurntSushi/toml | Config parsing |
| github.com/google/uuid | UUID generation |
| filippo.io/edwards25519 | Edwards curve arithmetic for FROST |
