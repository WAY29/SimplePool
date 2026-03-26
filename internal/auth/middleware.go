package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type contextKey string

const authenticatedContextKey contextKey = "authenticated"

func Middleware(service *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := parseBearerToken(header)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthorized",
				"message": "missing or invalid bearer token",
			})
			return
		}

		authenticated, err := service.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthorized",
				"message": "unauthorized",
			})
			return
		}

		c.Set(string(authenticatedContextKey), authenticated)
		c.Next()
	}
}

func Current(c *gin.Context) (*Authenticated, bool) {
	value, ok := c.Get(string(authenticatedContextKey))
	if !ok {
		return nil, false
	}

	authenticated, ok := value.(*Authenticated)
	return authenticated, ok
}

func parseBearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", false
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", false
	}

	return token, true
}
