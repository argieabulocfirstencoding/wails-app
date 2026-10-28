// Command update-manifest-sign creates and signs the stable B2 update manifest.
// It is used by GitHub Actions; -generate-key is intentionally manual so the
// private key can be stored in GitHub Secrets without entering the repository.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type signedManifest struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func main() {
	generateKey := flag.Bool("generate-key", false, "generate a new Ed25519 key pair")
	input := flag.String("input", "", "path to the unsigned manifest JSON")
	output := flag.String("output", "", "path to the signed manifest JSON")
	flag.Parse()

	if *generateKey {
		generate()
		return
	}
	if *input == "" || *output == "" {
		fatal("use -input stable-manifest.json -output latest.json")
	}
	privateKey, err := decodePrivateKey(os.Getenv("UPDATE_MANIFEST_SIGNING_KEY"))
	if err != nil {
		fatal(err.Error())
	}
	payload, err := os.ReadFile(*input)
	if err != nil {
		fatal(fmt.Sprintf("read manifest: %v", err))
	}
	if len(payload) == 0 {
		fatal("manifest is empty")
	}

	envelope, err := signManifest(payload, privateKey)
	if err != nil {
		fatal(fmt.Sprintf("encode signed manifest: %v", err))
	}
	if err := os.WriteFile(*output, envelope, 0o644); err != nil {
		fatal(fmt.Sprintf("write signed manifest: %v", err))
	}
}

func signManifest(payload []byte, privateKey ed25519.PrivateKey) ([]byte, error) {
	return json.Marshal(signedManifest{
		Payload:   base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload)),
	})
}

func generate() {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fatal(fmt.Sprintf("generate key: %v", err))
	}
	fmt.Println("Add this GitHub Actions variable: UPDATE_MANIFEST_PUBLIC_KEY")
	fmt.Println(base64.StdEncoding.EncodeToString(publicKey))
	fmt.Println()
	fmt.Println("Add this GitHub Actions secret: UPDATE_MANIFEST_SIGNING_KEY")
	fmt.Println(base64.StdEncoding.EncodeToString(privateKey))
	fmt.Fprintln(os.Stderr, "Store the private key only in GitHub Secrets or a password manager. Do not commit or share it.")
}

func decodePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("UPDATE_MANIFEST_SIGNING_KEY is not base64")
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("UPDATE_MANIFEST_SIGNING_KEY must decode to %d bytes", ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(decoded), nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "update-manifest-sign:", message)
	os.Exit(1)
}
