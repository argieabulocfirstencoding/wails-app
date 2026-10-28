package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSignManifestProducesVerifiableEnvelope(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"version":"2.6.0"}`)
	document, err := signManifest(payload, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	var envelope signedManifest
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	decodedPayload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if string(decodedPayload) != string(payload) || !ed25519.Verify(publicKey, decodedPayload, signature) {
		t.Fatal("signed manifest was not verifiable")
	}
}
