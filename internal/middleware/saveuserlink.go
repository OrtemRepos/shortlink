package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/internal/domain"
	"github.com/OrtemRepos/shortlink/internal/logger"
)

var log = logger.GetLogger()

const schema = `
CREATE TABLE IF NOT EXISTS user_links (
	user_id UUID,
	url_id SERIAL REFERENCES urls (id),
	PRIMARY KEY (user_id, url_id)
);
`

func InitTable(db *sqlx.DB) {
	log.Info("Initializing user_links table")
	db.MustExec(schema)
}

func saveUserLink(db *sqlx.DB, userID string, url *domain.URL) error {
	_, err := db.Exec(
		"INSERT INTO user_links (user_id, url_id) VALUES ($1, $2) ON CONFLICT (user_id, url_id) DO NOTHING",
		userID, url.ID,
	)
	if err != nil {
		log.Error("Failed to save user link",
			zap.Any("url", url),
			zap.String("user_id", userID),
			zap.Error(err))
	}
	return err
}

func SaveUserLinkMiddleware(db *sqlx.DB) gin.HandlerFunc {
	log.Info("SaveUserLinkMiddleware initialized")
	InitTable(db)
	return func(c *gin.Context) {
		log.Info("SaveUserLinkMiddleware called")
		userID := c.GetString("UserID")
		urls, exists := c.Get("urls")
		result := c.GetStringMap("result")
		if result == nil {
			result = make(map[string]interface{})
		}
		if !exists {
			result["error"] = "URL not found in context"
			c.AbortWithStatusJSON(http.StatusInternalServerError, result)
			return
		}

		urlList, ok := urls.([]*domain.URL)
		if !ok {
			result["error"] = "Invalid URL type in context"
			c.AbortWithStatusJSON(http.StatusInternalServerError, result)
			return
		}
		for _, url := range urlList {
			if err := saveUserLink(db, userID, url); err != nil {
				result["error"] = "Failed to save user link"
				c.AbortWithStatusJSON(http.StatusInternalServerError, result)
				return
			}
		}
		c.Set("result", result)
		c.Next()
	}
}
