package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/internal/logger"
	"github.com/OrtemRepos/shortlink/internal/ports"
)

var log = logger.GetLogger()

func AuthMiddleware(providerJWT ports.PortJWT) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := c.GetStringMap("result")
		if result == nil {
			result = make(map[string]interface{})
		}
		tokenString, err := c.Cookie("auth")
		if err != nil || tokenString == "" {
			log.Error("Authorization failed: no auth cookie", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization failed: no auth cookie"})
			return
		}

		claims, err := CheckToken(tokenString, providerJWT)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "BAD CREND"})
			return
		}
		if claims.UserID == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "Empty UserID"},
			)
			return
		}
		c.Set("claims", claims)
		c.Set("UserID", claims.UserID)
		result["UserID"] = claims.UserID
		c.Set("result", result)
		c.Next()
	}
}

// func LoginMiddleware(providerJWT ports.PortJWT) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		userID := uuid.NewString()
// 		tokenString, err := providerJWT.BuildJWTString(userID)
// 		if err != nil {
// 			log.Info("LoginMeddleware error", zap.Error(err))
// 			c.AbortWithError(http.StatusInternalServerError, err)
// 			return
// 		}
// 		log.Info("Set Cookie")
// 		c.Set("UserID", userID)
// 		c.SetCookie("auth", tokenString, int(time.Hour*10), "/", "", false, true)
// 	}
// }

func CheckToken(tokenString string, providerJWT ports.PortJWT) (*ports.Claims, error) {
	claims, err := providerJWT.GetClaims(tokenString)
	if err != nil {
		log.Error("Failed to validate token", zap.Error(err), zap.String("token", tokenString))
		return nil, err
	}
	log.Info("User authorized successfully", zap.Any("claims", claims))
	return claims, nil
}
