package middleware

import (
	"task-api/internal/config"
	"task-api/pkg/utils"
	"task-api/pkg/errors"
	"github.com/gin-gonic/gin"
	"strings"
	"github.com/golang-jwt/jwt/v5"
)

func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Error(customerrors.ErrUnauthorized)
			c.Abort()
			return
		}
		
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Error(customerrors.ErrUnauthorized)
			c.Abort()
			return
		}
		
		tokenStr := parts[1]
		token, err := utils.ValidateJWT(tokenStr, cfg.JWTSecret)
		if err != nil || !token.Valid {
			c.Error(customerrors.ErrUnauthorized)
			c.Abort()
			return
		}
		
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.Error(customerrors.ErrUnauthorized)
			c.Abort()
			return
		}
		
		userID := claims["user_id"].(string)
		c.Set("user_id", userID)
		c.Next()
	}
}
