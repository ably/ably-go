package proto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// CipherAlgorithm algorithms used for channel encryption.
type CipherAlgorithm uint

const (
	AES CipherAlgorithm = 1 << iota
)

func (c CipherAlgorithm) String() string {
	switch c {
	case AES:
		return "aes"
	default:
		return ""
	}
}

// CipherMode defines supported  ciphers.
type CipherMode uint

const (
	// CBC defines cbc mode.
	CBC CipherMode = 1 << iota
)

func (c CipherMode) String() string {
	switch c {
	case CBC:
		return "cbs"
	default:
		return ""
	}
}

const (
	DefaultKeyLength       = 256
	DefaultCipherAlgorithm = AES
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

	// This is a sequence of bytes used as initialization vector.This is used to
	// user distinct ciphertexts are produced even when the same plaintext is
	// encrypted multiple times independently with the same key.
	//
	// This field is optional. A random value will be generated if this is set to
	// nil.
	IV []byte

	// The cipher mode to be used for encryption default is CBC.
	//
	// Spec item (TZ2c)
	Mode CipherMode
}

// ChannelOptions defines options provided for creating a new channel.
type ChannelOptions struct {
	Cipher CipherParams
	cipher ChannelCipher
}

// GetCipher returns a ChannelCipher based on the algorithms set in the
// ChannelOptions.CipherParams.
func (c *ChannelOptions) GetCipher() (ChannelCipher, error) {
	if c.cipher != nil {
		return c.cipher, nil
	}
	switch c.Cipher.Algorithm {
	case AES:
		cipher, err := NewCBCCipher(c.Cipher)
		if err != nil {
			return nil, err
		}
		c.cipher = cipher
		return cipher, nil
	default:
		return nil, errors.New("unknown cipher algorithm")
	}
}

// ChannelCipher is an interface for encrypting and decrypting channel messages.
type ChannelCipher interface {
	Encrypt(plainText []byte) ([]byte, error)
	Decrypt(plainText []byte) ([]byte, error)
	GetAlgorithm() string
}

var _ ChannelCipher = (*CBCCipher)(nil)

// CBCCipher implements ChannelCipher that uses CBC mode.
type CBCCipher struct {
	algorithm string
	params    CipherParams
}

// NewCBCCipher returns a new CBCCipher that uses opts to initialize.
func NewCBCCipher(opts CipherParams) (*CBCCipher, error) {
	if opts.Algorithm != AES {
		return nil, errors.New("unknown cipher algorithm")
	}
	if opts.Mode != 0 && opts.Mode != CBC {
		return nil, errors.New("unknown cipher mode")
	}
	algo := fmt.Sprintf("cipher+%s-%d-cbc", opts.Algorithm, opts.KeyLength)
	return &CBCCipher{
		algorithm: algo,
		params:    opts,
	}, nil
}

// GenerateRandomKey returns a random key. keyLength is optional if provided it
// should be  in bits, it defaults to DefaultKeyLength when not provided.
//
// Spec RSE2, RSE2a, RSE2b.
func GenerateRandomKey(keyLength ...int) ([]byte, error) {
	length := DefaultKeyLength
	if len(keyLength) > 0 {
		length = keyLength[0]
	}
	key := make([]byte, length/8)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// DefaultCipherParams returns CipherParams with fields set to default values.
// This generates random secret key and iv values
func DefaultCipherParams() (*CipherParams, error) {
	c := &CipherParams{
		Algorithm: DefaultCipherAlgorithm,
	}
	c.Key = make([]byte, DefaultKeyLength/8)
	if _, err := io.ReadFull(rand.Reader, c.Key); err != nil {
		return nil, err
	}
	c.KeyLength = len(c.Key) * 8
	c.IV = make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, c.IV); err != nil {
		return nil, err
	}
	return c, nil
}

// Encrypt encrypts plainText using AES algorithm and returns encoded bytes.
func (c *CBCCipher) Encrypt(plainText []byte) ([]byte, error) {
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
	iv := c.params.IV
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
func (c *CBCCipher) Decrypt(cipherText []byte) ([]byte, error) {
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
func (c *CBCCipher) GetAlgorithm() string {
	return c.algorithm
}
