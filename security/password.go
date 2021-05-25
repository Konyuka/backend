package security

import (
	"golang.org/x/crypto/bcrypt"
)

// Hash password before save
func Hash(pass string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
}

// VerifyPassword to login
func VerifyPassword(hashedPassword, pass string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(pass))
}
