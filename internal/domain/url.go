package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"math/big"
	"strconv"
)

const maxInt64 = 1<<63 - 1

type URL struct {
	UUID        string `json:"-" db:"user_id"`
	ShortURL    string `json:"shortURL" db:"short_url"`
	OriginalURL string `json:"longURL" db:"original_url"`
	DeletedFlag bool   `json:"-" db:"is_deleted"`
}

func (u *URL) GenerateShortURL() string {
	randomInt, err := rand.Int(rand.Reader, big.NewInt(maxInt64))
	if err != nil {
		log.Printf("GenerateShortUrl: %s", err)
	}

	randomStr := strconv.FormatInt(randomInt.Int64(), 10)
	hash := sha256.Sum256([]byte(u.OriginalURL + randomStr))
	u.ShortURL = hex.EncodeToString(hash[:])[:8]
	return u.ShortURL
}

func NewURL(longURL string) *URL {
	return &URL{
		OriginalURL: longURL,
		ShortURL:    "",
	}
}
