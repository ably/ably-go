package proto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// CipherAlgorithm algorithms used for channel enccryption.
type CipherAlgorithm uint

const (
	// AES is the default channel encryption algorithm.
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

const (
	DefaultKeyLength       = 255
	DefaultCipherAlgorithm = AES
)

// CipherParams  provides parameters for configuring encryption  for channels.
//
//(TZ1)
type CipherParams struct {
	Algorithm CipherAlgorithm // (TZ2a)
	// The length of the private key in bits
	KeyLength int //(TZ2b)
	// This is the private key used to  encrypt/decrypt payloads.
	Key []byte //(TZ2d)

	IV []byte
}

// ChannelOptions defines options provided for creating a new channel.
type ChannelOptions struct {
	Cipher CipherParams
	cipher ChannelCipher
}

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

// CBCCipher implements ChannelCipher that uses cbc block cipher.
type CBCCipher struct {
	algorithm string
	params    CipherParams
}

// NewCBCCipher returns a new CBCCipher that uses opts to initialize.
func NewCBCCipher(opts CipherParams) (*CBCCipher, error) {
	if opts.Algorithm != AES {
		return nil, errors.New("unknown cipher algorithm")
	}
	algo := fmt.Sprintf("cipher+%s-%d-cbc", opts.Algorithm, opts.KeyLength)
	return &CBCCipher{
		algorithm: algo,
		params:    opts,
	}, nil
}

// DefaultCipherParams returns CipherParams with fields set to default values.
// This generates random secret key and iv values
func DefaultCipherParams() (*CipherParams, error) {
	c := &CipherParams{
		Algorithm: DefaultCipherAlgorithm,
	}
	c.Key = make([]byte, DefaultKeyLength)
	if _, err := io.ReadFull(rand.Reader, c.Key); err != nil {
		return nil, err
	}
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
	out := make([]byte, aes.BlockSize+len(plainText))
	copy(out[:aes.BlockSize], c.params.IV)
	cipher.NewCBCEncrypter(block, out[:aes.BlockSize]).CryptBlocks(out[aes.BlockSize:], plainText)
	return out, nil
}

// Decrypt decrypts cipherText using CBC cipher and AES algorithm and returns
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
