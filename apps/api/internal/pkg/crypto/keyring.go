package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Envelope format (binary, stored in the existing BYTEA columns):
//
//	"PaaS" (4)  magic
//	version (1) envelope format version (currently 0x01)
//	kek_ver (1) which KEK was used to wrap the DEK
//	dek_nonce (12)
//	wrapped_dek (32 + 16) AES-256-GCM(KEK[kek_ver], DEK)
//	data_nonce (12)
//	data_ciphertext + tag (N + 16) AES-256-GCM(DEK, plaintext)
//
// Minimum size = 94 bytes (empty plaintext). Shorter blobs are assumed to be
// legacy raw AES-256-GCM ciphertext for transparent backward compatibility.
var envelopeMagic = []byte{'P', 'a', 'a', 'S', 0x01}

const (
	envelopeHeaderLen = 6  // magic(5) + kek_ver(1)
	dekNonceLen       = 12
	wrappedDEKLen     = 48 // 32 key bytes + 16 GCM tag
	dataNonceLen      = 12
	gcmTagLen         = 16
	envelopeMinLen    = envelopeHeaderLen + dekNonceLen + wrappedDEKLen + dataNonceLen + gcmTagLen
	dekLen            = 32
	kekLen            = 32
)

// Keyring holds one or more AES-256 KEKs (Key Encryption Keys) and tracks
// which version is active for new encryptions. Each secret is encrypted with
// its own random DEK (Data Encryption Key); the DEK is wrapped by the current
// KEK. Rotation re-wraps the DEK without re-encrypting the underlying data.
type Keyring struct {
	keks           map[byte][]byte
	currentVersion byte
}

// NewKeyring constructs a Keyring from a map of version→raw-key and the
// version to use for new encryptions. All keys must be exactly 32 bytes.
func NewKeyring(keks map[byte][]byte, current byte) (*Keyring, error) {
	if len(keks) == 0 {
		return nil, errors.New("keyring: at least one KEK is required")
	}
	if _, ok := keks[current]; !ok {
		return nil, fmt.Errorf("keyring: current version %d not present in keyring", current)
	}
	copied := make(map[byte][]byte, len(keks))
	for v, k := range keks {
		if len(k) != kekLen {
			return nil, fmt.Errorf("keyring: KEK v%d must be %d bytes, got %d", v, kekLen, len(k))
		}
		copy := make([]byte, kekLen)
		for i := range k {
			copy[i] = k[i]
		}
		copied[v] = copy
	}
	return &Keyring{keks: copied, currentVersion: current}, nil
}

// ParseKeyringEnv builds a Keyring from the ENCRYPTION_KEYS / ENCRYPTION_KEY /
// ENCRYPTION_KEY_CURRENT env var trio.
//
//   - If encryptionKeys is non-empty, it is parsed as "v1:hex64,v2:hex64,...".
//   - Otherwise, encryptionKey (legacy single key) is treated as v1.
//   - If currentStr is empty, the highest version in the keyring is used.
func ParseKeyringEnv(encryptionKeys, encryptionKey, currentStr string) (*Keyring, error) {
	keks := make(map[byte][]byte)

	if strings.TrimSpace(encryptionKeys) != "" {
		entries := strings.Split(encryptionKeys, ",")
		for _, entry := range entries {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("keyring: invalid ENCRYPTION_KEYS entry %q (expected vN:hex)", entry)
			}
			versionStr := strings.TrimPrefix(strings.TrimSpace(parts[0]), "v")
			versionNum, err := strconv.Atoi(versionStr)
			if err != nil || versionNum < 1 || versionNum > 255 {
				return nil, fmt.Errorf("keyring: invalid version %q (must be v1..v255)", parts[0])
			}
			keyBytes, err := hex.DecodeString(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("keyring: decode hex for v%d: %w", versionNum, err)
			}
			if len(keyBytes) != kekLen {
				return nil, fmt.Errorf("keyring: v%d must be %d bytes (%d hex chars), got %d bytes", versionNum, kekLen, kekLen*2, len(keyBytes))
			}
			if _, dup := keks[byte(versionNum)]; dup {
				return nil, fmt.Errorf("keyring: duplicate version v%d", versionNum)
			}
			keks[byte(versionNum)] = keyBytes
		}
	} else if strings.TrimSpace(encryptionKey) != "" {
		keyBytes, err := hex.DecodeString(strings.TrimSpace(encryptionKey))
		if err != nil {
			return nil, fmt.Errorf("keyring: decode ENCRYPTION_KEY hex: %w", err)
		}
		if len(keyBytes) != kekLen {
			return nil, fmt.Errorf("keyring: ENCRYPTION_KEY must be %d bytes (%d hex chars)", kekLen, kekLen*2)
		}
		keks[1] = keyBytes
	}

	if len(keks) == 0 {
		return nil, errors.New("keyring: no keys configured (set ENCRYPTION_KEYS or ENCRYPTION_KEY)")
	}

	var current byte
	if strings.TrimSpace(currentStr) == "" {
		versions := make([]int, 0, len(keks))
		for v := range keks {
			versions = append(versions, int(v))
		}
		sort.Ints(versions)
		current = byte(versions[len(versions)-1])
	} else {
		versionStr := strings.TrimPrefix(strings.TrimSpace(currentStr), "v")
		versionNum, err := strconv.Atoi(versionStr)
		if err != nil || versionNum < 1 || versionNum > 255 {
			return nil, fmt.Errorf("keyring: invalid ENCRYPTION_KEY_CURRENT %q", currentStr)
		}
		current = byte(versionNum)
	}

	return NewKeyring(keks, current)
}

// CurrentVersion returns the KEK version used for new encryptions.
func (k *Keyring) CurrentVersion() byte { return k.currentVersion }

// Versions returns all configured KEK versions in ascending order.
func (k *Keyring) Versions() []byte {
	vs := make([]int, 0, len(k.keks))
	for v := range k.keks {
		vs = append(vs, int(v))
	}
	sort.Ints(vs)
	out := make([]byte, len(vs))
	for i, v := range vs {
		out[i] = byte(v)
	}
	return out
}

// Encrypt returns an envelope-format ciphertext sealed under the current KEK.
func (k *Keyring) Encrypt(plaintext []byte) ([]byte, error) {
	return k.encryptWithVersion(plaintext, k.currentVersion)
}

func (k *Keyring) encryptWithVersion(plaintext []byte, version byte) ([]byte, error) {
	kek, ok := k.keks[version]
	if !ok {
		return nil, fmt.Errorf("keyring: unknown KEK version %d", version)
	}

	dek := make([]byte, dekLen)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}

	dekNonce, wrappedDEK, err := sealGCM(kek, dek)
	if err != nil {
		return nil, fmt.Errorf("wrap DEK: %w", err)
	}
	dataNonce, dataCT, err := sealGCM(dek, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt data: %w", err)
	}

	buf := make([]byte, 0, envelopeHeaderLen+dekNonceLen+len(wrappedDEK)+dataNonceLen+len(dataCT))
	buf = append(buf, envelopeMagic...)
	buf = append(buf, version)
	buf = append(buf, dekNonce...)
	buf = append(buf, wrappedDEK...)
	buf = append(buf, dataNonce...)
	buf = append(buf, dataCT...)
	return buf, nil
}

// Decrypt handles both envelope and legacy raw AES-256-GCM ciphertexts. Legacy
// blobs are decrypted using KEK v1 (the only key available before the keyring
// existed).
func (k *Keyring) Decrypt(ciphertext []byte) ([]byte, error) {
	if isEnvelope(ciphertext) {
		return k.decryptEnvelope(ciphertext)
	}
	return k.decryptLegacy(ciphertext)
}

// Rewrap decrypts a ciphertext (legacy or envelope) and re-encrypts it with
// the current KEK version. Safe to run on already-current rows: the output is
// fresh envelope ciphertext tagged with the current version.
func (k *Keyring) Rewrap(ciphertext []byte) ([]byte, error) {
	plaintext, err := k.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("rewrap decrypt: %w", err)
	}
	return k.Encrypt(plaintext)
}

// IsCurrent reports whether the given envelope ciphertext is already tagged
// with the current KEK version. Legacy ciphertexts are never current.
func (k *Keyring) IsCurrent(ciphertext []byte) bool {
	if !isEnvelope(ciphertext) {
		return false
	}
	return ciphertext[5] == k.currentVersion
}

func isEnvelope(ciphertext []byte) bool {
	if len(ciphertext) < envelopeMinLen {
		return false
	}
	return bytes.Equal(ciphertext[:len(envelopeMagic)], envelopeMagic)
}

func (k *Keyring) decryptEnvelope(ct []byte) ([]byte, error) {
	if len(ct) < envelopeMinLen {
		return nil, errors.New("envelope: ciphertext too short")
	}
	kekVer := ct[5]
	kek, ok := k.keks[kekVer]
	if !ok {
		return nil, fmt.Errorf("envelope: unknown KEK version %d", kekVer)
	}
	offset := envelopeHeaderLen
	dekNonce := ct[offset : offset+dekNonceLen]
	offset += dekNonceLen
	wrappedDEK := ct[offset : offset+wrappedDEKLen]
	offset += wrappedDEKLen
	dataNonce := ct[offset : offset+dataNonceLen]
	offset += dataNonceLen
	dataCT := ct[offset:]

	dek, err := openGCM(kek, dekNonce, wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w", err)
	}
	defer zero(dek)

	plaintext, err := openGCM(dek, dataNonce, dataCT)
	if err != nil {
		return nil, fmt.Errorf("decrypt data: %w", err)
	}
	return plaintext, nil
}

func (k *Keyring) decryptLegacy(ct []byte) ([]byte, error) {
	kek, ok := k.keks[1]
	if !ok {
		return nil, errors.New("legacy: KEK v1 required to decrypt legacy ciphertext but not present")
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("legacy: create cipher: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("legacy: create GCM: %w", err)
	}
	if len(ct) < aesGCM.NonceSize() {
		return nil, errors.New("legacy: ciphertext too short")
	}
	nonce, body := ct[:aesGCM.NonceSize()], ct[aesGCM.NonceSize():]
	return aesGCM.Open(nil, nonce, body, nil)
}

// sealGCM returns (nonce, ciphertext+tag) after AES-256-GCM sealing.
func sealGCM(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return nonce, aesGCM.Seal(nil, nonce, plaintext, nil), nil
}

func openGCM(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aesGCM.Open(nil, nonce, ciphertext, nil)
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
