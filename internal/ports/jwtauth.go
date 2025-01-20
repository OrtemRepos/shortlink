package ports

import (
	"github.com/golang-jwt/jwt/v5"
)

type PortJWT interface {
	BuildJWTString(id string) (string, error)
	GetClaims(tokenString string) (*Claims, error)
}
type Claims struct {
	jwt.RegisteredClaims
	UserID string
}
