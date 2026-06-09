package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEd25519PrivateKey_Raw64B(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	got, err := loadEd25519PrivateKey(priv, "")
	if err != nil {
		t.Fatalf("loadEd25519PrivateKey err = %v", err)
	}
	if !got.Equal(priv) {
		t.Error("private key mismatch")
	}
}

func TestLoadEd25519PrivateKey_Raw32BSeed(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	expected := ed25519.NewKeyFromSeed(seed)
	got, err := loadEd25519PrivateKey(seed, "")
	if err != nil {
		t.Fatalf("loadEd25519PrivateKey err = %v", err)
	}
	if !got.Equal(expected) {
		t.Error("private key mismatch from seed")
	}
}

func TestLoadEd25519PrivateKey_PEM(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	tmp := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(tmp, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadEd25519PrivateKey(nil, tmp)
	if err != nil {
		t.Fatalf("loadEd25519PrivateKey (file) err = %v", err)
	}
	if !got.Equal(priv) {
		t.Error("private key mismatch from PEM file")
	}
}

func TestLoadEd25519PrivateKey_Empty(t *testing.T) {
	_, err := loadEd25519PrivateKey(nil, "")
	if err != ErrAAPPrivateKeyEmpty {
		t.Errorf("err = %v, want ErrAAPPrivateKeyEmpty", err)
	}
}

func TestLoadEd25519PrivateKey_BadLength(t *testing.T) {
	_, err := loadEd25519PrivateKey([]byte("too-short"), "")
	if err == nil {
		t.Error("expected error for bad length")
	}
}

func TestLoadEd25519PrivateKey_BadPEM(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(tmp, []byte("not-pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadEd25519PrivateKey(nil, tmp)
	if err == nil {
		t.Error("expected error for bad PEM")
	}
}

func TestLoadEd25519PrivateKey_MissingFile(t *testing.T) {
	_, err := loadEd25519PrivateKey(nil, "/no/such/file")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
