package user

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	minUsernameLength = 3
	maxUsernameLength = 255
	minPasswordLength = 8
	maxPasswordLength = 72 // bcrypt input limit
)

// Username is a Value Object with its own validation rules.
type Username string

func NewUsername(s string) (Username, error) {
	u := Username(strings.TrimSpace(s))
	if len(u) < minUsernameLength || len(u) > maxUsernameLength {
		return "", ErrInvalidUsername
	}
	return u, nil
}

func (u Username) String() string {
	return string(u)
}

// ValidatePassword enforces the password policy on the plaintext password.
// The policy is a business rule; hashing stays an application concern.
func ValidatePassword(password string) error {
	if l := len(password); l < minPasswordLength || l > maxPasswordLength {
		return ErrInvalidPassword
	}
	return nil
}

// User is the aggregate root of the identity context.
// PasswordHash stores the bcrypt hash, never the plaintext.
type User struct {
	ID           string
	Username     Username
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewUser(username Username, passwordHash string) *User {
	now := time.Now()
	return &User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
