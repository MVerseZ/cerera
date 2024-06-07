package storage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"

	"github.com/cerera/internal/cerera/types"
)

// LoadFromFile loads encrypted data from a JSON file into the vault.
func LoadFromFile(filename string, key []byte) error {
	var v = GetVault()

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	decryptedData, err := decrypt(data, key)
	if err != nil {
		return err
	}

	err = json.Unmarshal(decryptedData, &v.accounts)
	if err != nil {
		return err
	}

	return nil
}

// SaveToFile encrypts and saves data from the vault to a JSON file.
func SaveToFile(filename string, key []byte, data []byte) error {

	var vault = GetVault()
	for _, v := range vault.accounts.accounts {
		var buf, _ = json.Marshal(v)

		encryptedData, err := encrypt(buf, key)
		if err != nil {
			return err
		}

		err = os.WriteFile(filename, encryptedData, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

func SavePair(address types.Address, data []byte) {
	// if file not exist we will create it
	f, err := os.OpenFile("./vault.dat", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// check if address as key exists

	// write data
	var buf = bytes.Buffer{}
	buf.Write(address[:])
	buf.Write(data)
	f.Write(buf.Bytes())
}
