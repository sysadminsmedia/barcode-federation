# FBS JSON Schema Definitions

This directory contains JSON Schema definitions for all FBS-1.0 (Federated Barcode System version 1.0) data types and protocol messages.

All schemas conform to [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12/schema) and validate against RFC FBS0001 specifications.

## Directory Structure

Schemas are organized to mirror their published HTTPS URL hierarchy:

```
metadata/
├── core/v1.0.0.json                          https://schemas.fbs-community.example/metadata/core/v1.0.0.json
├── apparel/v1.0.0.json                       https://schemas.fbs-community.example/metadata/apparel/v1.0.0.json
├── book/v1.0.0.json                          https://schemas.fbs-community.example/metadata/book/v1.0.0.json
├── electronics/v1.0.0.json                   https://schemas.fbs-community.example/metadata/electronics/v1.0.0.json
├── food/v1.0.0.json                          https://schemas.fbs-community.example/metadata/food/v1.0.0.json
├── logistics/v1.0.0.json                     https://schemas.fbs-community.example/metadata/logistics/v1.0.0.json
├── music/v1.0.0.json                         https://schemas.fbs-community.example/metadata/music/v1.0.0.json
├── pharmaceutical/v1.0.0.json                https://schemas.fbs-community.example/metadata/pharmaceutical/v1.0.0.json
└── video/v1.0.0.json                         https://schemas.fbs-community.example/metadata/video/v1.0.0.json
lifecycle/
└── shipment-tracking/v1.0.0.json             https://schemas.fbs-community.example/lifecycle/shipment-tracking/v1.0.0.json
types/
├── actor-profile/v1.0.0.json                 https://schemas.fbs-community.example/types/actor-profile/v1.0.0.json
├── barcode-record/v1.0.0.json                https://schemas.fbs-community.example/types/barcode-record/v1.0.0.json
├── federation-message/v1.0.0.json            https://schemas.fbs-community.example/types/federation-message/v1.0.0.json
├── key-succession-declaration/v1.0.0.json    https://schemas.fbs-community.example/types/key-succession-declaration/v1.0.0.json
├── namespace-claim/v1.0.0.json               https://schemas.fbs-community.example/types/namespace-claim/v1.0.0.json
├── node-compromise-notice/v1.0.0.json        https://schemas.fbs-community.example/types/node-compromise-notice/v1.0.0.json
├── node-meta/v1.0.0.json                     https://schemas.fbs-community.example/types/node-meta/v1.0.0.json
├── outbox-page/v1.0.0.json                   https://schemas.fbs-community.example/types/outbox-page/v1.0.0.json
├── peer-list/v1.0.0.json                     https://schemas.fbs-community.example/types/peer-list/v1.0.0.json
└── verification-badge/v1.0.0.json            https://schemas.fbs-community.example/types/verification-badge/v1.0.0.json
```

This structure allows:
- **Version separation**: Multiple versions of the same schema (v1.0.0, v1.1.0, v2.0.0) can coexist
- **URL mirroring**: Filesystem paths exactly match published schema URLs
- **Clear organization**: Protocol types grouped under `types/`, metadata schemas under `metadata/`, lifecycle schemas under `lifecycle/`

## Schema Files

### Core Data Types

#### `barcode-record/v1.0.0.json`
**Barcode Record** — The primary data unit in FBS representing a barcode and its associated product metadata.

- Defined in: RFC FBS0001 Section 4.2.1
- Required fields: `fbs`, `type`, `id`, `cbi`, `symbology`, `value`, `status`, `submittedBy`, `submittedAt`, `updatedAt`, `originNode`, `metadataSchema`, `metadata`, `revisionHistory`, `signature`
- HTTPS URL: `https://schemas.fbs-community.example/types/barcode-record/v1.0.0.json`

Key concepts:
- **CBI (Canonical Barcode Identifier)**: Uniform format `{symbology}:{value}` for all barcode references
- **Namespace Tracking**: Supports both GS1-track (numeric values) and self-sovereign (FBS URI) barcodes
- **Immutable Provenance**: `submittedBy`, `submittedAt`, `originNode` never change across revisions
- **Revision History**: Append-only log of all changes with cryptographic chain linking via `$defs/RevisionHistoryEntry`
- **Status Lifecycle**: Tracks record state as `active`, `retracted`, or `expired`
- **Inline Sub-schemas**: `$defs` for `NamespaceClaim`, `GS1CrossRef`, and `RevisionHistoryEntry`

#### `namespace-claim/v1.0.0.json`
**Namespace Claim** — A signed assertion that an actor has issuing or controlling authority over a barcode range/namespace.

- Defined in: RFC FBS0001 Section 4.2.3
- Required fields: `fbs`, `type`, `id`, `claimedBy`, `namespaceType`, `scope`, `issuedAt`, `evidence`, `signature`
- HTTPS URL: `https://schemas.fbs-community.example/types/namespace-claim/v1.0.0.json`

Key concepts:
- **Namespace Types**: `gs1` (GS1 company prefixes) or `self` (self-sovereign via cryptographic keypair)
- **Scope**: Defines extent of claimed authority (prefix ranges for GS1, FNI for self-sovereign)
- **Evidence**: Supports multiple verification methods (GS1 registry, DNS, business registration, trademark)
- **Expiration**: Required for GS1 claims, optional for self-sovereign claims

#### `actor-profile/v1.0.0.json`
**Actor Profile** — Metadata document for an FBS actor describing identity, public key, and capability set.

- Defined in: RFC FBS0001 Section 4.1.3
- Exposed at: `https://{node-host}/.well-known/fbs/actors/{actor-local-id}` and `https://{node-host}/.well-known/fbs/webrecord?actor={actor-local-id}`
- Required fields: `fbs`, `type`, `id`, `node`, `publicKey`, `capabilities`
- HTTPS URL: `https://schemas.fbs-community.example/types/actor-profile/v1.0.0.json`

Key concepts:
- **Actor Identifier (AID)**: Format `fbs://{node-host}/{actor-local-id}` identifying an actor globally
- **Capabilities**: Atomic access control primitives determining allowed operations
- **Built-in Profiles**: PublicUser, RetailOperator, Manufacturer (plus arbitrary custom profiles)
- **Verification Badges**: Optional for actors with GS1-track namespace claims

#### `verification-badge/v1.0.0.json`
**Verification Badge** — A signed assertion that a manufacturer actor has been verified at a specified level (0-3).

- Defined in: RFC FBS0001 Section 4.5.3
- Required fields: `fbs`, `type`, `badgeId`, `level`, `subject`, `subjectKeyFingerprint`, `issuedBy`, `issuedAt`, `expiresAt`, `claims`, `signature`
- HTTPS URL: `https://schemas.fbs-community.example/types/verification-badge/v1.0.0.json`

Key concepts:
- **Verification Levels**: Level 0 (unverified) through Level 3 (quorum-attested)
- **Quorum Details**: For Level 3, includes threshold, participants, and evidence sources via `$defs/QuorumDetails`
- **Transparency Log**: SCT and merge deadline for Level 3 badges via `$defs/TransparencyLogRef`
- **Key Binding**: `subjectKeyFingerprint` binds badge to a specific actor keypair

#### `key-succession-declaration/v1.0.0.json`
**Key Succession Declaration** — A dual-signed document authorising a successor namespace keypair to retract records signed under the old keypair.

- Defined in: RFC FBS0001 Section 4.2.5.7.2
- Required fields: `fbs`, `type`, `id`, `oldFni`, `oldPublicKey`, `successorFni`, `successorPublicKey`, `scope`, `declaredAt`, `expiresAt`, `oldKeySignature`, `successorKeySignature`
- HTTPS URL: `https://schemas.fbs-community.example/types/key-succession-declaration/v1.0.0.json`

Key concepts:
- **Dual Signatures**: Signed by both old and new namespace keypairs to prove simultaneous control
- **Retract-Only Scope**: Successor key authority is limited to issuing retractions for old records
- **Expiration Window**: Bounds the time during which successor key may retract old records (typically 30-90 days)

### Federation Protocol

#### `federation-message/v1.0.0.json`
**Federation Message** — HTTP POST message delivered to peer node inboxes to propagate records and execute federation activities.

- Defined in: RFC FBS0001 Section 4.4.2
- Endpoint: `POST https://{node-host}/federation/inbox`
- Required fields: `fbs`, `type`, `id`, `sender`, `recipient`, `activity`, `publishedAt`, `signature`
- HTTPS URL: `https://schemas.fbs-community.example/types/federation-message/v1.0.0.json`

Key concepts:
- **Activity Types**: `Publish`, `Retract`, `ClaimPublish`, `ClaimRetract`, `KeySuccessionDeclaration`, `KeyRotation`, `NodeCompromiseNotice`, `TransparencyAlert`, `Ping`, `Pong`
- **Object Payload**: Varies by activity type; may be a full Barcode Record, lightweight reference, or null
- **HTTP Signatures**: All federation messages signed by originating node's private key (RFC 9421)
- **At-Least-Once Delivery**: Nodes MUST implement exponential backoff retry (min 30s, max 24h)

#### `node-compromise-notice/v1.0.0.json`
**Node Compromise Notice** — The payload object of a `NodeCompromiseNotice` federation activity, announcing that a node's keypair has been compromised.

- Defined in: RFC FBS0001 Section 4.4.3
- Required fields: `type`, `affectedNodeId`, `affectedKeyFingerprint`, `compromisedAfter`, `newPublicKey`, `notice`, `publishedBy`
- HTTPS URL: `https://schemas.fbs-community.example/types/node-compromise-notice/v1.0.0.json`

Key concepts:
- **Compromise Timestamp**: `compromisedAfter` marks the earliest point at which the attacker may have had access
- **New Key Distribution**: `newPublicKey` allows peers to trust the new key immediately
- **Quarantine**: Receiving nodes MUST quarantine messages from the affected node signed after the compromise timestamp

#### `outbox-page/v1.0.0.json`
**Outbox Page** — A paginated page of federation messages from a node's outbox endpoint.

- Defined in: RFC FBS0001 Section 4.4.5
- Required fields: `fbs`, `type`, `nodeId`, `totalItems`, `page`, `pageSize`, `items`
- HTTPS URL: `https://schemas.fbs-community.example/types/outbox-page/v1.0.0.json`

### Node Discovery & Metadata

#### `node-meta/v1.0.0.json`
**Node Metadata** — Information about an FBS node's capabilities, federation policies, and public key.

- Defined in: RFC FBS0001 Section 4.3.1
- Exposed at: `https://{node-host}/.well-known/fbs/meta`
- Required fields: `fbs`, `type`, `nodeId`, `displayName`, `publicKey`, `inbox`
- HTTPS URL: `https://schemas.fbs-community.example/types/node-meta/v1.0.0.json`

Key concepts:
- **Federation Policy**: `open` (any peer), `allowlist` (explicit peers only), or `blocklist` (all except explicit deny)
- **Inbox/Outbox**: Symmetric federation endpoints for receiving and exposing federated messages
- **Public Key**: Node's Ed25519 key for signing federation messages and trust establishment
- **Capabilities**: Advertised feature support (federation, bulk-import, subscriptions, etc.)
- **Trust Anchors**: Recognized verification badge issuers this node trusts

#### `peer-list/v1.0.0.json`
**Peer List** — Advisory list of known FBS peers exposed at well-known endpoint for network bootstrapping.

- Defined in: RFC FBS0001 Section 4.3.4
- Exposed at: `https://{node-host}/.well-known/fbs/peers`
- Required fields: `fbs`, `type`, `nodeId`, `publishedAt`, `ttl`, `peers`
- HTTPS URL: `https://schemas.fbs-community.example/types/peer-list/v1.0.0.json`

Key concepts:
- **Time-to-Live (TTL)**: Cache duration in seconds (1-24 hours); consumers MUST NOT use expired data
- **Peer Relationships**: `federated` (actively exchanging records) or `known` (aware but not federating)
- **Peer Sub-schema**: Each peer entry validated via `$defs/Peer` with `nodeId`, `meta`, `addedAt`, `relationship`
- **Anti-Poisoning**: Implementations SHOULD enforce timeouts on metadata fetches and limit new peers per cycle

### Metadata Schemas

#### `metadata/core/v1.0.0.json`
**FBS Core Metadata Schema** — Base schema that all domain-specific metadata schemas MUST incorporate via `allOf` reference.

- Defined in: RFC FBS0001 Section 4.2.4.3
- HTTPS URL: `https://schemas.fbs-community.example/metadata/core/v1.0.0.json`
- Required fields in all domain schemas: `name`, `languages`

Key concepts:
- **Cross-Schema Interoperability**: Common fields guaranteed present across all barcode records
- **Language Support**: BCP 47 language tags; all language-dependent content provided in first listed language
- **Images**: Product images with semantic roles (primary, label, packaging, detail, alternate)
- **Extensibility**: `additionalProperties: true` allows domain schemas to add arbitrary fields

#### `metadata/food/v1.0.0.json`
**FBS Food Product Metadata Schema** — For food products with nutritional information, ingredients, and allergens.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/food/v1.0.0.json`
- Additional required fields: `ingredients`
- Key fields: `servingSize`, `nutritionFacts` (with multi-standard support), `allergens` (EU 1169/2011), `certifications`, `storageInstructions`

#### `metadata/book/v1.0.0.json`
**FBS Book Metadata Schema** — For books and printed publications.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/book/v1.0.0.json`
- Additional required fields: `authors`
- Key fields: `isbn10`, `isbn13`, `publisher`, `publishedDate`, `edition`, `pageCount`, `format`, `genres`, `series`, `synopsis`, `targetAudience`, `deweyDecimal`, `locClassification`

#### `metadata/music/v1.0.0.json`
**FBS Music Album Metadata Schema** — For CDs, vinyl records, and other music releases.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/music/v1.0.0.json`
- Additional required fields: `artists`
- Key fields: `label`, `catalogNumber`, `format` (cd/vinyl-lp/cassette/etc.), `tracks` (with per-track details), `totalDuration`, `isrc`, `compilation`, `explicit`

#### `metadata/video/v1.0.0.json`
**FBS Video Media Metadata Schema** — For DVDs, Blu-rays, and other video media.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/video/v1.0.0.json`
- Additional required fields: `format`
- Key fields: `contentType` (film/series/documentary), `rating` (MPAA/BBFC/FSK/etc.), `audioTracks` (with codec/channel info), `subtitleLanguages`, `region`, `aspectRatio`, `specialFeatures`

#### `metadata/pharmaceutical/v1.0.0.json`
**FBS Pharmaceutical Product Metadata Schema** — For medications and pharmaceutical products.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/pharmaceutical/v1.0.0.json`
- Additional required fields: `activeIngredients`, `dosageForm`
- Key fields: `routeOfAdministration`, `ndcNumber`, `atcCode`, `prescriptionRequired`, `warnings`, `regulatoryApprovals` (FDA/EMA/MHRA), `controlledSubstanceSchedule`

#### `metadata/electronics/v1.0.0.json`
**FBS Consumer Electronics Metadata Schema** — For consumer electronics products.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/electronics/v1.0.0.json`
- Additional required fields: `manufacturer`, `modelNumber`
- Key fields: `category` (smartphone/laptop/etc.), `dimensions`, `connectivity`, `powerSupply`, `displayInfo`, `certifications` (FCC/CE/UL), `operatingSystem`, `storageCapacity`

#### `metadata/logistics/v1.0.0.json`
**FBS Logistics Shipment Metadata Schema** -- For logistics shipments with tracking, carrier, and delivery information.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/logistics/v1.0.0.json`
- Additional required fields: `shipmentType`, `trackingStatus`
- Key fields: `carrier`, `trackingNumber`, `weight`, `dimensions`, `originFacility`, `destinationFacility`, `trackingStatus`, `currentLocation`, `estimatedDelivery`, `lastScannedAt`, `handoffPartner`, `contents`, `serviceLevel`, `temperatureRange`, `handlingInstructions`

#### `metadata/apparel/v1.0.0.json`
**FBS Apparel Metadata Schema** — For clothing and textile products.

- HTTPS URL: `https://schemas.fbs-community.example/metadata/apparel/v1.0.0.json`
- Additional required fields: `materials`
- Key fields: `garmentType`, `size`, `sizeSystem` (US/UK/EU/JP), `gender`, `careInstructions`, `materials` (with composition percentages), `sustainabilityCertifications`

### Lifecycle Schemas

#### `lifecycle/shipment-tracking/v1.0.0.json`
**FBS Shipment Tracking Lifecycle Schema** -- Defines which fields may be updated via the `record:lifecycle` capability for shipment tracking records.

- HTTPS URL: `https://schemas.fbs-community.example/lifecycle/shipment-tracking/v1.0.0.json`
- Required fields: `trackingStatus`
- Updatable fields: `trackingStatus`, `currentLocation`, `lastScannedAt`, `estimatedDelivery`, `handoffPartner`, `deliveryInstructions`

Key concepts:
- **Scoped Updates**: Only fields declared in the lifecycle schema may be modified after initial record creation
- **Record Integrity**: Lifecycle updates are appended to revision history like any other change
- **Capability Gated**: Requires the `record:lifecycle` capability on the actor profile

## Schema Validation

All barcode records MUST validate against BOTH:
1. Their declared domain-specific metadata schema (via `metadataSchema` URL)
2. The FBS Core Schema (implicitly via `allOf` in domain schemas)

Example validation in pseudocode:
```
barcode_record:
  - validate(record, barcode-record.schema.json)  -> Structure and signatures
  - fetch(record.metadataSchema) -> domain_schema
  - validate(record.metadata, domain_schema)       -> Domain-specific rules
  - validate(record.metadata, core-metadata.schema.json)  -> Core interop fields
```

## Schema Pinning & Immutability

- Schema URLs MUST be stable and version-specific (e.g., `.../food/v1.2.0.json`)
- Once published at a version URL, schema content MUST NEVER CHANGE
- Corrections require publishing a new version URL
- Mutable "latest" URLs are allowed only when `metadataSchemaDigest` is present
- Digest format: `sha256:{64-hex-character-hash}`

## Protocol Version

All schemas in this directory target **FBS Protocol Version 1.0** as defined in [RFC FBS0001](../rfc/FBS0001.md).

- `fbs` field: MUST be `"1.0"` for all FBS objects
- Schema identifiers use `/v1.0.0/` in their `$id` URLs
- Future versions will use distinct version numbers (v1.1.0, v2.0.0, etc.)

## References

- [RFC FBS0001 - Federated Barcode System](../rfc/FBS0001.md) — Complete protocol specification
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12/schema) — Validation standard
- [RFC 9421 - HTTP Message Signatures](https://tools.ietf.org/html/rfc9421) — Message authentication
- [RFC 9591 - FROST Threshold Signatures](https://tools.ietf.org/html/rfc9591) — Quorum signing
- [RFC 9162 - Certificate Transparency](https://tools.ietf.org/html/rfc9162) — Verification logging
- [BCP 47 - Language Tags](https://tools.ietf.org/html/bcp47) — Language identification
- [ISO 3166-1 - Country Codes](https://www.iso.org/iso-3166-1-alpha-2.html) — Country identification

## Contributing

When modifying these schemas:

1. **Preserve backward compatibility** — Do not break existing valid documents
2. **Update version identifiers** — Increment the version in `$id` URLs if any change
3. **Reference RFC FBS0001** — Cite specific sections justifying additions/removals
4. **Validate thoroughly** — Test against complete example documents from the RFC
5. **Update this README** — Document schema changes and implications
