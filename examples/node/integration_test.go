package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fbscommunity/barcode-federation/examples/node/api"
	"github.com/fbscommunity/barcode-federation/examples/node/config"
	fbscrypto "github.com/fbscommunity/barcode-federation/examples/node/crypto"
	"github.com/fbscommunity/barcode-federation/examples/node/federation"
	"github.com/fbscommunity/barcode-federation/examples/node/quorum"
	"github.com/fbscommunity/barcode-federation/examples/node/store"
	"github.com/fbscommunity/barcode-federation/examples/node/transparencylog"
)

// testNode wraps a running test server with its configuration.
type testNode struct {
	server    *httptest.Server
	srv       *api.Server
	db        *store.Store
	dbPath    string
	nodeKey   []byte // ed25519.PrivateKey
}

func newTestNode(t *testing.T, host string) *testNode {
	t.Helper()

	dbPath := t.TempDir() + "/test.db"
	db, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	_, nodeKey, err := fbscrypto.GenerateKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	cfg := &config.Config{
		Node: config.NodeConfig{
			Host:        host,
			DisplayName: "Test Node " + host,
			OperatedBy:  "Test",
		},
		Federation: config.FederationConfig{
			Policy: "open",
		},
	}

	srv := api.NewServer(cfg, db, nodeKey)

	// Set up transparency log
	_, logKey, _ := fbscrypto.GenerateKeypair()
	tlog, _ := transparencylog.NewTransparencyLog("https://"+host+"/fbs-log", logKey)
	srv.TransparencyLog = tlog

	// Set up quorum coordinator
	coordinator := quorum.NewCoordinator("https://"+host, nodeKey, tlog)
	srv.QuorumCoordinator = coordinator

	// Set up federation
	nodeBaseURL := "https://" + host
	dispatcher := federation.NewDispatcher(db, nodeKey, nodeBaseURL)
	srv.Dispatcher = dispatcher
	srv.FederationInboxHandler = &federation.InboxHandler{
		Store:       db,
		NodeBaseURL: nodeBaseURL,
		PolicyCheck: func(string) bool { return true },
	}
	srv.FederationOutboxHandler = &federation.OutboxHandler{
		Store:       db,
		NodeBaseURL: nodeBaseURL,
	}

	ts := httptest.NewServer(srv.Router)
	// Override the host to match the test server's actual URL
	// (the config host is for generating IDs, but HTTP goes to ts.URL)

	return &testNode{
		server:  ts,
		srv:     srv,
		db:      db,
		dbPath:  dbPath,
		nodeKey: nodeKey,
	}
}

func (tn *testNode) close() {
	tn.server.Close()
	tn.db.Close()
	os.Remove(tn.dbPath)
}

func (tn *testNode) createActor(t *testing.T, localID, profile string, caps []string) string {
	t.Helper()

	pub, actorPriv, _ := fbscrypto.GenerateKeypair()
	actorPubPEM, _ := fbscrypto.EncodePublicKeyPEM(pub)
	actorPrivPEM, _ := fbscrypto.EncodePrivateKeyPEM(actorPriv)

	capsJSON, _ := json.Marshal(caps)
	host := tn.srv.Config.Node.Host
	token := fmt.Sprintf("test-token-%s-%d", localID, time.Now().UnixNano())

	actor := &store.ActorForCreate{
		ID:              "fbs://" + host + "/" + localID,
		Profile:         profile,
		DisplayName:     localID,
		Node:            "https://" + host,
		PublicKeyPem:    actorPubPEM,
		CapabilitiesRaw: string(capsJSON),
		PrivateKeyPem:   actorPrivPEM,
	}

	if err := tn.db.CreateActorWithKey(actor, token); err != nil {
		t.Fatalf("create actor %s: %v", localID, err)
	}
	return token
}

func (tn *testNode) get(t *testing.T, path string) (int, map[string]interface{}) {
	return tn.getAuth(t, path, "")
}

func (tn *testNode) getAuth(t *testing.T, path, token string) (int, map[string]interface{}) {
	t.Helper()
	req, _ := http.NewRequest("GET", tn.server.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return parseResponse(t, resp)
}

func (tn *testNode) post(t *testing.T, path string, body interface{}, token string) (int, map[string]interface{}) {
	t.Helper()
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tn.server.URL+path, bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return parseResponse(t, resp)
}

func (tn *testNode) put(t *testing.T, path string, body interface{}, token string) (int, map[string]interface{}) {
	t.Helper()
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", tn.server.URL+path, bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return parseResponse(t, resp)
}

func parseResponse(t *testing.T, resp *http.Response) (int, map[string]interface{}) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result) // ok if this fails for non-JSON responses
	return resp.StatusCode, result
}

// --- Tests ---

func TestIntegration_NodeMetadata(t *testing.T) {
	node := newTestNode(t, "test-node.example.com")
	defer node.close()

	status, body := node.get(t, "/.well-known/fbs/meta")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %v", status, body)
	}
	if body["fbs"] != "1.0" {
		t.Error("fbs should be 1.0")
	}
	if body["type"] != "NodeMeta" {
		t.Error("type should be NodeMeta")
	}
	if body["displayName"] != "Test Node test-node.example.com" {
		t.Errorf("unexpected displayName: %v", body["displayName"])
	}
	t.Log("Node metadata endpoint works")
}

func TestIntegration_ActorProfile(t *testing.T) {
	node := newTestNode(t, "actor-test.example.com")
	defer node.close()

	node.createActor(t, "acme-corp", "Manufacturer", []string{
		"record:read", "record:submit", "namespace:claim",
	})

	status, body := node.get(t, "/.well-known/fbs/actors/acme-corp")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %v", status, body)
	}
	if body["type"] != "Actor" {
		t.Error("type should be Actor")
	}

	// Also test webrecord endpoint
	status2, body2 := node.get(t, "/.well-known/fbs/webrecord?actor=acme-corp")
	if status2 != 200 {
		t.Fatalf("webrecord: expected 200, got %d", status2)
	}
	if body2["id"] != body["id"] {
		t.Error("both endpoints should return the same actor")
	}
	t.Log("Actor profile endpoints work")
}

func TestIntegration_RecordLifecycle(t *testing.T) {
	node := newTestNode(t, "records-test.example.com")
	defer node.close()

	token := node.createActor(t, "manufacturer", "Manufacturer", []string{
		"record:read", "record:submit", "record:update:own", "record:retract:own",
	})

	// Submit a record
	status, body := node.post(t, "/api/v1/records", map[string]interface{}{
		"symbology":      "ean-13",
		"value":          "5901234123457",
		"metadataSchema": "https://schemas.fbs-community.example/metadata/food/v1.0.0.json",
		"metadata": map[string]interface{}{
			"name":        "Test Oats 500g",
			"languages":   []string{"en"},
			"brand":       "TestBrand",
			"ingredients": []string{"Oats"},
		},
	}, token)

	if status != 201 {
		t.Fatalf("submit: expected 201, got %d: %v", status, body)
	}
	recordID, _ := body["id"].(string)
	if recordID == "" {
		t.Fatal("record should have an ID")
	}
	if body["cbi"] != "ean-13:5901234123457" {
		t.Errorf("unexpected cbi: %v", body["cbi"])
	}
	if body["status"] != "active" {
		t.Errorf("status should be active, got %v", body["status"])
	}
	if body["signature"] == nil || body["signature"] == "" {
		t.Error("record should be signed")
	}
	t.Logf("Record submitted: %s", recordID)

	// Search for it
	status, body = node.get(t, "/api/v1/barcodes?q=5901234123457")
	if status != 200 {
		t.Fatalf("search: expected 200, got %d", status)
	}
	total, _ := body["total"].(float64)
	if total < 1 {
		t.Error("search should find at least 1 record")
	}

	// Lookup by CBI
	status, body = node.get(t, "/api/v1/barcodes/ean-13/5901234123457")
	if status != 200 {
		t.Fatalf("lookup: expected 200, got %d", status)
	}
	if body["best"] == nil {
		t.Error("lookup should return a best record")
	}

	// Duplicate submission should fail
	status, _ = node.post(t, "/api/v1/records", map[string]interface{}{
		"symbology":      "ean-13",
		"value":          "5901234123457",
		"metadataSchema": "https://schemas.example.com/food/v1.0.0.json",
		"metadata":       map[string]interface{}{"name": "Duplicate", "languages": []string{"en"}},
	}, token)
	if status != 400 {
		t.Errorf("duplicate should return 400, got %d", status)
	}

	t.Log("Record lifecycle (submit, search, lookup, duplicate rejection) works")
}

func TestIntegration_AuthAndCapabilities(t *testing.T) {
	node := newTestNode(t, "auth-test.example.com")
	defer node.close()

	// No auth should fail
	status, _ := node.post(t, "/api/v1/records", map[string]interface{}{}, "")
	if status != 401 {
		t.Errorf("no auth should return 401, got %d", status)
	}

	// Bad token should fail
	status, _ = node.post(t, "/api/v1/records", map[string]interface{}{}, "bad-token")
	if status != 401 {
		t.Errorf("bad token should return 401, got %d", status)
	}

	// PublicUser without record:submit:bulk should fail bulk import
	token := node.createActor(t, "viewer", "PublicUser", []string{"record:read", "record:submit"})
	status, body := node.post(t, "/api/v1/bulk-import", []interface{}{}, token)
	if status != 403 {
		t.Errorf("PublicUser bulk import should return 403, got %d: %v", status, body)
	}

	t.Log("Auth and capability enforcement works")
}

func TestIntegration_PeerList(t *testing.T) {
	node := newTestNode(t, "peers-test.example.com")
	defer node.close()

	// Add a peer
	node.db.AddPeer("https://peer1.example.com", "https://peer1.example.com/.well-known/fbs/meta", "federated")
	node.db.AddPeer("https://peer2.example.com", "https://peer2.example.com/.well-known/fbs/meta", "known")

	status, body := node.get(t, "/.well-known/fbs/peers")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	if body["type"] != "PeerList" {
		t.Error("type should be PeerList")
	}
	peers, _ := body["peers"].([]interface{})
	if len(peers) != 2 {
		t.Errorf("expected 2 peers, got %d", len(peers))
	}
	t.Log("Peer list endpoint works")
}

func TestIntegration_TransparencyLog(t *testing.T) {
	node := newTestNode(t, "tlog-test.example.com")
	defer node.close()

	// Check metadata
	status, body := node.get(t, "/fbs-log/v1/metadata")
	if status != 200 {
		t.Fatalf("metadata: expected 200, got %d: %v", status, body)
	}
	if body["type"] != "TransparencyLogMeta" {
		t.Errorf("unexpected type: %v", body["type"])
	}

	// Submit a badge
	badge := map[string]interface{}{
		"fbs":     "1.0",
		"type":    "VerificationBadge",
		"badgeId": "https://test/badges/badge-001",
		"level":   2,
		"subject": "fbs://test/actors/acme",
	}
	status, body = node.post(t, "/fbs-log/v1/add-badge", map[string]interface{}{
		"badge":       badge,
		"submitterId": "https://test/trust-anchor",
	}, "")
	if status != 200 {
		t.Fatalf("add-badge: expected 200, got %d: %v", status, body)
	}
	if body["signature"] == nil || body["signature"] == "" {
		t.Error("SCT should have a signature")
	}
	badgeHash, _ := body["badgeHash"].(string)
	if badgeHash == "" {
		t.Fatal("SCT should have badgeHash")
	}
	t.Logf("Badge submitted, SCT received: hash=%s", badgeHash)

	// Get signed tree head
	status, body = node.get(t, "/fbs-log/v1/get-sth")
	if status != 200 {
		t.Fatalf("get-sth: expected 200, got %d", status)
	}
	treeSize, _ := body["treeSize"].(float64)
	if treeSize != 1 {
		t.Errorf("tree size should be 1, got %v", treeSize)
	}

	// Get entries
	status, body = node.get(t, "/fbs-log/v1/get-entries?start=0&end=10")
	if status != 200 {
		t.Fatalf("get-entries: expected 200, got %d", status)
	}
	entries, _ := body["entries"].([]interface{})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	// Get inclusion proof
	status, body = node.get(t, "/fbs-log/v1/get-proof-by-hash?hash="+badgeHash)
	if status != 200 {
		t.Fatalf("get-proof: expected 200, got %d: %v", status, body)
	}
	leafIndex, _ := body["leafIndex"].(float64)
	if leafIndex != 0 {
		t.Errorf("leaf index should be 0, got %v", leafIndex)
	}

	t.Log("Transparency log (submit, SCT, STH, entries, proof) works")
}

func TestIntegration_QuorumVerification(t *testing.T) {
	node := newTestNode(t, "quorum-test.example.com")
	defer node.close()

	token := node.createActor(t, "acme-mfr", "Manufacturer", []string{
		"record:read", "record:submit", "namespace:claim",
	})

	// Start verification
	status, body := node.post(t, "/api/v1/verification/quorum", map[string]interface{}{
		"actorId":         "fbs://quorum-test.example.com/acme-mfr",
		"claimedPrefixes": []string{"5901234"},
		"evidencePackage": map[string]interface{}{
			"gs1PrefixLicense":     "LICENSE-12345",
			"businessRegistration": "REG-67890",
			"verifiedDomain":       "acme.example.com",
		},
	}, token)

	if status != 202 {
		t.Fatalf("start verification: expected 202, got %d: %v", status, body)
	}
	sessionID, _ := body["sessionId"].(string)
	if sessionID == "" {
		t.Fatal("should return sessionId")
	}
	threshold, _ := body["threshold"].(float64)
	if threshold < 2 {
		t.Errorf("threshold should be >= 2, got %v", threshold)
	}
	t.Logf("Quorum session started: %s (threshold=%v)", sessionID, threshold)

	// Poll for completion (the simulated verifiers run async)
	var finalBody map[string]interface{}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, finalBody = node.getAuth(t, "/api/v1/verification/quorum/"+sessionID, token)
		if status != 200 {
			t.Fatalf("poll: expected 200, got %d", status)
		}
		sessionStatus, _ := finalBody["status"].(string)
		if sessionStatus == "completed" || sessionStatus == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	sessionStatus, _ := finalBody["status"].(string)
	if sessionStatus == "failed" {
		t.Fatalf("quorum session failed: %v", finalBody["error"])
	}
	if sessionStatus != "completed" {
		t.Fatalf("expected completed, got %s (after 10s timeout)", sessionStatus)
	}

	// Should have a badge
	if finalBody["badge"] == nil {
		t.Fatal("completed session should return the badge")
	}

	badgeMap, ok := finalBody["badge"].(map[string]interface{})
	if !ok {
		t.Fatal("badge should be a JSON object")
	}
	if badgeMap["level"] != float64(3) {
		t.Errorf("badge level should be 3, got %v", badgeMap["level"])
	}
	if badgeMap["signature"] == nil || badgeMap["signature"] == "" {
		t.Error("badge should have FROST threshold signature")
	}

	// Check quorum details in badge
	claims, _ := badgeMap["claims"].(map[string]interface{})
	if claims != nil {
		quorumDetails, _ := claims["quorum"].(map[string]interface{})
		if quorumDetails == nil {
			t.Error("badge claims should have quorum details")
		} else {
			qThreshold, _ := quorumDetails["threshold"].(float64)
			if qThreshold < 2 {
				t.Errorf("quorum threshold should be >= 2, got %v", qThreshold)
			}
			evidenceSources, _ := quorumDetails["evidenceSources"].([]interface{})
			if len(evidenceSources) < int(qThreshold) {
				t.Errorf("should have >= threshold evidence sources, got %d", len(evidenceSources))
			}
		}
	}

	// Check transparency log reference
	tlogRef, _ := badgeMap["transparencyLog"].(map[string]interface{})
	if tlogRef == nil {
		t.Error("Level 3 badge should have transparency log reference")
	} else if tlogRef["sct"] == nil || tlogRef["sct"] == "" {
		t.Error("transparency log ref should have SCT")
	}

	t.Log("Full quorum verification ceremony (request -> evaluate -> FROST sign -> transparency log -> badge) works")
}

func TestIntegration_FederationOutbox(t *testing.T) {
	node := newTestNode(t, "outbox-test.example.com")
	defer node.close()

	status, body := node.get(t, "/federation/outbox")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %v", status, body)
	}
	if body["type"] != "OutboxPage" {
		t.Errorf("type should be OutboxPage, got %v", body["type"])
	}
	t.Log("Federation outbox endpoint works")
}

func TestIntegration_Subscriptions(t *testing.T) {
	node := newTestNode(t, "sub-test.example.com")
	defer node.close()

	token := node.createActor(t, "retailer", "RetailOperator", []string{
		"record:read", "record:submit", "subscription:create",
	})

	status, body := node.post(t, "/api/v1/subscriptions", map[string]interface{}{
		"scope": map[string]interface{}{
			"symbology": "ean-13",
			"prefixes":  []string{"590"},
		},
		"callbackUrl": "https://retailer.example.com/webhooks/fbs",
		"events":      []string{"create", "update", "retract"},
	}, token)

	if status != 201 {
		t.Fatalf("expected 201, got %d: %v", status, body)
	}
	if body["id"] == nil || body["id"] == "" {
		t.Error("subscription should have an ID")
	}
	t.Log("Subscription creation works")
}

func TestIntegration_NamespaceClaims(t *testing.T) {
	node := newTestNode(t, "claims-test.example.com")
	defer node.close()

	token := node.createActor(t, "mfr", "Manufacturer", []string{
		"record:read", "record:submit", "namespace:claim",
	})

	// Submit a GS1 namespace claim
	status, body := node.post(t, "/api/v1/claims", map[string]interface{}{
		"namespaceType": "gs1",
		"scope": map[string]interface{}{
			"symbologies": []string{"ean-13"},
			"prefixes":    []string{"5901234"},
		},
		"expiresAt": "2028-01-15T00:00:00Z",
		"evidence": map[string]interface{}{
			"type":             "GS1CompanyPrefix",
			"verifiedPrefixes": []string{"5901234"},
			"verificationMethod": "GS1GEPIR",
		},
	}, token)

	if status != 201 {
		t.Fatalf("expected 201, got %d: %v", status, body)
	}
	if body["type"] != "NamespaceClaim" {
		t.Errorf("type should be NamespaceClaim, got %v", body["type"])
	}
	if body["signature"] == nil || body["signature"] == "" {
		t.Error("claim should be signed")
	}

	// List claims
	status, _ = node.get(t, "/api/v1/actors/mfr/claims")
	if status != 200 {
		t.Fatalf("list claims: expected 200, got %d", status)
	}
	t.Log("Namespace claims (submit + list) works")
}

func TestIntegration_CustodyTransferAndDelegation(t *testing.T) {
	node := newTestNode(t, "custody-test.example.com")
	defer node.close()

	// Create FedEx (submitter) and USPS (delegate) actors
	fedexToken := node.createActor(t, "fedex", "LogisticsOperator", []string{
		"record:read", "record:submit", "record:update:own", "record:delegate", "record:encrypt",
	})
	uspsToken := node.createActor(t, "usps", "LogisticsOperator", []string{
		"record:read", "record:submit", "record:update:own", "record:lifecycle",
	})

	// FedEx submits a shipment record
	status, body := node.post(t, "/api/v1/records", map[string]interface{}{
		"symbology":      "qr-code",
		"value":          "fbs://fni1testnamespace/SHIP-001",
		"metadataSchema": "https://schemas.fbs-community.example/metadata/logistics/v1.0.0.json",
		"expiresAt":      "2026-06-01T00:00:00Z",
		"metadata": map[string]interface{}{
			"name":           "Shipment SHIP-001",
			"languages":      []string{"en"},
			"brand":          "FedEx",
			"shipmentType":   "B2C",
			"trackingStatus": "in-transit",
		},
		"relatedRecords": []map[string]interface{}{
			{"cbi": "ean-13:5901234567890", "relationship": "contains", "quantity": 1, "description": "Acme Speaker"},
		},
	}, fedexToken)

	if status != 201 {
		t.Fatalf("submit: expected 201, got %d: %v", status, body)
	}
	recordID, _ := body["id"].(string)
	t.Logf("FedEx submitted shipment: %s", recordID)

	// Verify related records are queryable
	status, body = node.get(t, "/api/v1/related/ean-13/5901234567890")
	if status != 200 {
		t.Fatalf("related: expected 200, got %d: %v", status, body)
	}
	total, _ := body["total"].(float64)
	if total < 1 {
		t.Error("should find at least 1 record related to the product CBI")
	}
	t.Log("Related records query works")

	// USPS tries to update the record (should fail — not owner or delegate)
	status, _ = node.put(t, "/api/v1/records/"+extractUUID(recordID), map[string]interface{}{
		"summary":  "USPS attempting unauthorized update",
		"metadata": map[string]interface{}{"name": "Shipment SHIP-001", "languages": []string{"en"}, "trackingStatus": "delivered"},
	}, uspsToken)
	if status != 403 {
		t.Errorf("USPS update without delegation should return 403, got %d", status)
	}
	t.Log("Unauthorized delegate update correctly rejected")

	// FedEx initiates custody transfer to USPS
	status, body = node.post(t, "/api/v1/custody/transfer", map[string]interface{}{
		"recordId":             recordID,
		"toActor":              "fbs://custody-test.example.com/usps",
		"condition":            "accepted",
		"delegateCapabilities": []string{"record:update:own", "record:lifecycle"},
		"expiresAt":            "2026-06-01T00:00:00Z",
	}, fedexToken)

	if status != 201 {
		t.Fatalf("custody transfer: expected 201, got %d: %v", status, body)
	}
	t.Log("FedEx initiated custody transfer to USPS")

	// USPS accepts the custody transfer
	status, body = node.post(t, "/api/v1/custody/accept", map[string]interface{}{
		"recordId":         recordID,
		"transferNoticeId": "test",
		"condition":        "accepted",
	}, uspsToken)
	if status != 200 {
		t.Fatalf("custody accept: expected 200, got %d: %v", status, body)
	}
	t.Log("USPS accepted custody")

	// Now USPS can update the record as a delegate
	status, body = node.put(t, "/api/v1/records/"+extractUUID(recordID), map[string]interface{}{
		"summary": "Delivered by USPS carrier",
		"metadata": map[string]interface{}{
			"name":           "Shipment SHIP-001",
			"languages":      []string{"en"},
			"brand":          "FedEx",
			"shipmentType":   "B2C",
			"trackingStatus": "delivered",
		},
	}, uspsToken)
	if status != 200 {
		t.Fatalf("USPS delegate update: expected 200, got %d: %v", status, body)
	}
	updatedStatus, _ := body["status"].(string)
	if updatedStatus != "active" {
		t.Errorf("record should still be active, got %s", updatedStatus)
	}
	t.Log("USPS successfully updated record as delegate")

	// Verify custody chain
	status, body = node.get(t, "/api/v1/records/"+extractUUID(recordID)+"/custody")
	if status != 200 {
		t.Fatalf("custody chain: expected 200, got %d: %v", status, body)
	}
	chainLen, _ := body["chainLength"].(float64)
	if chainLen < 1 {
		t.Error("custody chain should have at least 1 entry")
	}
	t.Logf("Custody chain has %v entries", chainLen)

	t.Log("Full custody transfer flow (submit -> transfer -> accept -> delegate update -> chain query) works")
}

func extractUUID(recordID string) string {
	// Extract UUID from "https://host/api/v1/records/UUID"
	parts := splitLast(recordID, "/")
	return parts
}

func splitLast(s, sep string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if string(s[i]) == sep {
			return s[i+1:]
		}
	}
	return s
}
