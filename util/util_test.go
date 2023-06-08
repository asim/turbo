package util

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGetHash(t *testing.T) {
	password := "myPassword123"
	hash, err := GetHash(password)
	if err != nil {
		t.Fatalf("GetHash(%q) failed: %v", password, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Fatalf("GetHash(%q) = %q, does not match original password: %v", password, hash, err)
	}
}

func TestPassword(t *testing.T) {
	passwordLength := 10
	password := Password(passwordLength)

	if len(password) != passwordLength {
		t.Errorf("Password(%d) = %q, length does not match %d", passwordLength, password, passwordLength)
	}

	for _, c := range password {
		if !strings.Contains(alphanum, string(c)) {
			t.Errorf("Password(%d) = %q, contains invalid character %q", passwordLength, password, c)
		}
	}
}
