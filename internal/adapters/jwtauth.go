package adapters

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/configs"
	"github.com/OrtemRepos/shortlink/internal/logger"
	"github.com/OrtemRepos/shortlink/internal/ports"
)

type ProviderJWT struct {
	tokenExp  time.Duration
	log       *zap.Logger
	secretKey string
}

func NewProviderJWT(cfg *configs.Config) *ProviderJWT {
	return &ProviderJWT{
		tokenExp:  time.Duration(cfg.Auth.TokenExp),
		secretKey: cfg.Auth.SecretKey,
		log:       logger.GetLogger(),
	}
}

var ErrNotValidToken = errors.New("not valid token")

func (pj *ProviderJWT) BuildJWTString(id string) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		ports.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(pj.tokenExp)),
			},
			UserID: id,
		},
	)
	tokenString, err := token.SignedString([]byte(pj.secretKey))
	if err != nil {
		pj.log.Error("Failed to sign token", zap.Error(err))
		return "", err
	}

	return tokenString, nil
}

func (pj *ProviderJWT) GetClaims(tokenString string) (*ports.Claims, error) {
	claims := &ports.Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpecred signing method %v", t.Header["alg"])
			}
			return []byte(pj.secretKey), nil
		},
	)
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, ErrNotValidToken
	}

	return claims, nil
}
