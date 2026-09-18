package utils

import (
	"github.com/golang-jwt/jwt/v5"
	"time"
)

func GenerateJWT(userID string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateJWT(tokenStr string, secret string) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
}

// JWTIssuer issues signed JWT access tokens. It adapts the JWT library to
// the TokenIssuer port required by the auth usecase.
type JWTIssuer struct {
	Secret string
}

func (i JWTIssuer) Issue(userID string) (string, error) {
	return GenerateJWT(userID, i.Secret)
}
