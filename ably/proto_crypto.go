package ably

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// CipherAlgorithm is a supported algorithm for channel encryption.
type CipherAlgorithm uint

const (
	CipherAES CipherAlgorithm = 1 + iota
)

func (c CipherAlgorithm) String() string {
	switch c {
	case CipherAES:
		return "aes"
	default:
		return ""
	}
}

func (c CipherAlgorithm) isValidKeyLength(l int) bool {
	switch c {
	case CipherAES:
		return l == 128 || l == 256
	default:
		return false
	}
}

// CipherMode is a supported cipher mode for channel encryption.
type CipherMode uint

const (
	CipherCBC CipherMode = 1 + iota
)

func (c CipherMode) String() string {
	switch c {
	case CipherCBC:
		return "cbs"
	default:
		return ""
	}
}

const (
	defaultCipherKeyLength = 256
	defaultCipherAlgorithm = CipherAES
	defaultCipherMode      = CipherCBC
)

// CipherParams  provides parameters for configuring encryption  for channels.
//
//Spec item (TZ1)
type CipherParams struct {
	Algorithm CipherAlgorithm // Spec item (TZ2a)
	// The length of the private key in bits
	KeyLength int // Spec item (TZ2b)
	// This is the private key used to  encrypt/decrypt payloads.
	Key []byte // Spec item (TZ2d)
	// The cipher mode to be used for encryption default is CBC.
	//
	// Spec item (TZ2c)
	Mode CipherMode

	// iv is the initialization vector. Used only for comparing resulting
	// ciphertext with test fixtures; production code should always use a random
	// unique IV per encrypted message.
	iv []byte
}

// GetCipher returns a ChannelCipher based on the algorithms set in the
// ChannelOptions.CipherParams.
func (c *protoChannelOptions) GetCipher() (channelCipher, error) {
	if c == nil {
		return nil, errors.New("no cipher configured")
	}
	if c.cipher != nil {
		return c.cipher, nil
	}
	switch c.Cipher.Algorithm {
	case CipherAES:
		encoder, err := newCBCCipher(c.Cipher)
		if err != nil {
			return nil, err
		}
		c.cipher = encoder
		return encoder, nil
	default:
		return nil, errors.New("unknown cipher algorithm")
	}
}

// channelCipher is an interface for encrypting and decrypting channel messages.
type channelCipher interface {
	Encrypt(plainText []byte) ([]byte, error)
	Decrypt(plainText []byte) ([]byte, error)
	GetAlgorithm() string
}

var _ channelCipher = (*cbcCipher)(nil)

// cbcCipher implements ChannelCipher that uses CBC mode.
type cbcCipher struct {
	algorithm string
	params    CipherParams
}

// newCBCCipher returns a new CBCCipher that uses opts to initialize.
func newCBCCipher(opts CipherParams) (*cbcCipher, error) {
	if opts.Algorithm != CipherAES {
		return nil, errors.New("unknown cipher algorithm")
	}
	if opts.Mode != 0 && opts.Mode != CipherCBC {
		return nil, errors.New("unknown cipher mode")
	}
	algo := fmt.Sprintf("cipher+%s-%d-cbc", opts.Algorithm, opts.KeyLength)
	return &cbcCipher{
		algorithm: algo,
		params:    opts,
	}, nil
}

// DefaultCipherParams returns CipherParams with fields set to default values.
// This generates random secret key and iv values
func DefaultCipherParams() (*CipherParams, error) {
	c := &CipherParams{
		Algorithm: defaultCipherAlgorithm,
	}
	c.Key = make([]byte, defaultCipherKeyLength/8)
	if _, err := io.ReadFull(rand.Reader, c.Key); err != nil {
		return nil, err
	}
	c.KeyLength = len(c.Key) * 8
	return c, nil
}

// Encrypt encrypts plainText using AES algorithm and returns encoded bytes.
func (c *cbcCipher) Encrypt(plainText []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.params.Key)
	if err != nil {
		return nil, err
	}
	// Apply padding to the payload if it's not already padded. Event if
	// len(m.Data)%aes.Block == 0 we have no guarantee it's already padded.
	// Try to unpad it and pad on failure.
	if _, err := pkcs7Unpad(plainText, aes.BlockSize); err != nil {
		data, err := pkcs7Pad(plainText, aes.BlockSize)
		if err != nil {
			return nil, err
		}
		plainText = data
	}
	iv := c.params.iv
	if iv == nil {
		iv = make([]byte, aes.BlockSize)
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			return nil, err
		}
	}
	out := make([]byte, aes.BlockSize+len(plainText))
	copy(out[:aes.BlockSize], iv)
	cipher.NewCBCEncrypter(block, out[:aes.BlockSize]).CryptBlocks(out[aes.BlockSize:], plainText)
	return out, nil
}

// Decrypt decrypts cipherText using CBC mode and AES algorithm and returns
// decrypted bytes.
func (c *cbcCipher) Decrypt(cipherText []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.params.Key)
	if err != nil {
		return nil, err
	}
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}
	iv := []byte(cipherText[:aes.BlockSize])
	cipherText = cipherText[aes.BlockSize:]
	out := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, cipherText)
	out, err = pkcs7Unpad(out, aes.BlockSize)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetAlgorithm returns the cipher algorithm used by this CBCCipher which is AES.
func (c *cbcCipher) GetAlgorithm() string {
	return c.algorithm
}

// Appends padding.
func pkcs7Pad(data []byte, blocklen int) ([]byte, error) {
	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}
	padlen := 1
	for ((len(data) + padlen) % blocklen) != 0 {
		padlen = padlen + 1
	}
	p := make([]byte, len(data)+padlen)
	copy(p, data)
	for i := len(data); i < len(p); i++ {
		p[i] = byte(padlen)
	}
	return p, nil
}

// Returns slice of the original data without padding.
func pkcs7Unpad(data []byte, blocklen int) ([]byte, error) {
	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}
	if len(data)%blocklen != 0 || len(data) == 0 {
		return nil, fmt.Errorf("invalid data len %d", len(data))
	}
	padlen := int(data[len(data)-1])
	if padlen > blocklen || padlen == 0 {
		// no padding found.
		return data, nil
	}
	// check padding
	for _, p := range data[len(data)-padlen:] {
		if p != byte(padlen) {
			return nil, fmt.Errorf("invalid padding character")
		}
	}
	return data[:len(data)-padlen], nil
}
