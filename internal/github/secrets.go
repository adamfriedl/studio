package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/box"
)

// ActionsPublicKey is a repo's Actions secrets encryption key.
type ActionsPublicKey struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"` // base64-encoded 32-byte X25519 public key
}

// GetActionsPublicKey fetches the public key used to encrypt Actions secrets.
func (c *Client) GetActionsPublicKey(ctx context.Context, owner, repo string) (*ActionsPublicKey, error) {
	var out ActionsPublicKey
	path := fmt.Sprintf("/repos/%s/%s/actions/secrets/public-key", owner, repo)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	if out.KeyID == "" || out.Key == "" {
		return nil, fmt.Errorf("github actions public-key: empty key_id or key")
	}
	return &out, nil
}

// PutActionsSecret creates or updates a repository Actions secret.
// value is the plaintext secret; it is sealed with the repo public key (libsodium sealed box).
func (c *Client) PutActionsSecret(ctx context.Context, owner, repo, name, value string) error {
	pk, err := c.GetActionsPublicKey(ctx, owner, repo)
	if err != nil {
		return err
	}
	encrypted, err := SealActionsSecret(pk.Key, value)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", owner, repo, name)
	return c.do(ctx, http.MethodPut, path, map[string]string{
		"encrypted_value": encrypted,
		"key_id":          pk.KeyID,
	}, nil)
}

// SealActionsSecret encrypts plaintext for GitHub Actions secrets (crypto_box_seal).
func SealActionsSecret(publicKeyBase64, plaintext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("public key: want 32 bytes, got %d", len(raw))
	}
	var recipient [32]byte
	copy(recipient[:], raw)

	ephemeralPub, ephemeralPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	var nonce [24]byte
	h, err := blake2b.New(24, nil)
	if err != nil {
		return "", err
	}
	_, _ = h.Write(ephemeralPub[:])
	_, _ = h.Write(recipient[:])
	copy(nonce[:], h.Sum(nil))

	out := make([]byte, 0, 32+len(plaintext)+box.Overhead)
	out = append(out, ephemeralPub[:]...)
	out = box.Seal(out, []byte(plaintext), &nonce, &recipient, ephemeralPriv)
	return base64.StdEncoding.EncodeToString(out), nil
}
