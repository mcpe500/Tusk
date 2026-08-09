package artifact

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

const ManifestSchemaVersion = 1

type Manifest struct {
	SchemaVersion  int        `json:"schemaVersion"`
	ReleaseVersion string     `json:"releaseVersion"`
	GeneratedAt    string     `json:"generatedAt"`
	KeyID          string     `json:"keyId"`
	Artifacts      []Artifact `json:"artifacts"`
	Signature      string     `json:"signature"`
}

type Artifact struct {
	ID           string `json:"id"`
	Backend      string `json:"backend"`
	Architecture string `json:"architecture"`
	URL          string `json:"url"`
	MediaType    string `json:"mediaType"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}

type signedPayload struct {
	SchemaVersion  int        `json:"schemaVersion"`
	ReleaseVersion string     `json:"releaseVersion"`
	GeneratedAt    string     `json:"generatedAt"`
	KeyID          string     `json:"keyId"`
	Artifacts      []Artifact `json:"artifacts"`
}

func (m Manifest) Verify(publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return errors.New("invalid Ed25519 public key size")
	}
	if err := m.Validate(); err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("decode manifest signature: %w", err)
	}
	payload, err := m.signingBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return errors.New("manifest signature verification failed")
	}
	return nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("unsupported manifest schema %d", m.SchemaVersion)
	}
	if m.ReleaseVersion == "" || m.GeneratedAt == "" || m.KeyID == "" {
		return errors.New("manifest releaseVersion, generatedAt, and keyId are required")
	}
	if len(m.Artifacts) == 0 {
		return errors.New("manifest has no artifacts")
	}
	seen := make(map[string]struct{}, len(m.Artifacts))
	for index, artifact := range m.Artifacts {
		if err := artifact.Validate(); err != nil {
			return fmt.Errorf("artifact %d: %w", index, err)
		}
		if _, ok := seen[artifact.ID]; ok {
			return fmt.Errorf("duplicate artifact ID %q", artifact.ID)
		}
		seen[artifact.ID] = struct{}{}
	}
	if m.Signature == "" {
		return errors.New("manifest signature is required")
	}
	return nil
}

func (a Artifact) Validate() error {
	if a.ID == "" || a.Backend == "" || a.Architecture == "" || a.MediaType == "" {
		return errors.New("id, backend, architecture, and mediaType are required")
	}
	parsedURL, err := url.Parse(a.URL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return fmt.Errorf("artifact URL must be absolute HTTPS: %q", a.URL)
	}
	if a.Size <= 0 {
		return errors.New("artifact size must be positive")
	}
	if len(a.SHA256) != sha256.Size*2 {
		return errors.New("artifact SHA-256 must contain 64 hexadecimal characters")
	}
	digest, err := hex.DecodeString(a.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return errors.New("artifact SHA-256 is invalid")
	}
	if hex.EncodeToString(digest) != a.SHA256 {
		return errors.New("artifact SHA-256 must use lowercase hexadecimal")
	}
	return nil
}

func (m Manifest) signingBytes() ([]byte, error) {
	payload := signedPayload{
		SchemaVersion:  m.SchemaVersion,
		ReleaseVersion: m.ReleaseVersion,
		GeneratedAt:    m.GeneratedAt,
		KeyID:          m.KeyID,
		Artifacts:      m.Artifacts,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode signed manifest payload: %w", err)
	}
	return data, nil
}

// Sign is intended for the release tooling and tests, not runtime key custody.
func (m *Manifest) Sign(privateKey ed25519.PrivateKey) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid Ed25519 private key size")
	}
	payload, err := m.signingBytes()
	if err != nil {
		return err
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return nil
}
