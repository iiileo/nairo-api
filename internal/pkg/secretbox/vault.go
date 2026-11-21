package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const prefix = "enc:v1:"

// Vault 对称加密敏感密钥（App Secret / API Secret）落库；验签前再解密。
// key 为空时原样存取（仅允许 local 开发）。
type Vault struct {
	gcm cipher.AEAD
}

func New(masterKey string) (*Vault, error) {
	masterKey = strings.TrimSpace(masterKey)
	if masterKey == "" {
		return &Vault{}, nil
	}
	sum := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{gcm: gcm}, nil
}

func (v *Vault) Enabled() bool {
	return v != nil && v.gcm != nil
}

func (v *Vault) Seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if !v.Enabled() {
		return plain, nil
	}
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := v.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.RawURLEncoding.EncodeToString(ct), nil
}

// Open 解密；兼容历史明文（无 enc:v1: 前缀）。
func (v *Vault) Open(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, prefix) {
		return stored, nil
	}
	if !v.Enabled() {
		return "", fmt.Errorf("encrypted secret requires SECURITY_MASTER_KEY")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", err
	}
	ns := v.gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := v.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
