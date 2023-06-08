package util

import (
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	alphanum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

func Key(id string) string {
	hasher := fnv.New128()
	hasher.Write([]byte(id))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func GetHash(pwd string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.MinCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// generate a passworf of i length alphanum string
func Password(i int) string {
	bytes := make([]byte, i)
	for {
		rand.Read(bytes)
		for i, b := range bytes {
			bytes[i] = alphanum[b%byte(len(alphanum))]
		}
		return string(bytes)
	}
	return uuid.New().String()
}
