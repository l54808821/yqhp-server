package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// 默认加密密钥（生产环境应从配置读取）
var defaultKey = []byte("yqhp-gulu-secret-key-32bytes!!ab")

// Encrypt AES加密
func Encrypt(plaintext string) (string, error) {
	return EncryptWithKey(plaintext, defaultKey)
}

// Decrypt AES解密
func Decrypt(ciphertext string) (string, error) {
	return DecryptWithKey(ciphertext, defaultKey)
}

// EncryptWithKey 使用指定密钥进行AES加密
func EncryptWithKey(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 创建随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Base64编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptWithKey 使用指定密钥进行AES解密
func DecryptWithKey(ciphertext string, key []byte) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Base64解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// IsSameAfterEncryption 检查加密后的值是否与原值不同
func IsSameAfterEncryption(original, encrypted string) bool {
	return original == encrypted
}
