package httpapi

import (
	"errors"
	"net/http"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func registerAuthRoutes(engine *gin.Engine, service *auth.Service) {
	group := engine.Group("/api/auth")
	group.POST("/login", func(c *gin.Context) {
		var request loginRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "bad_request",
				"message": "invalid login payload",
			})
			return
		}

		result, err := service.Login(c.Request.Context(), auth.LoginInput{
			Username: request.Username,
			Password: request.Password,
		})
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    "invalid_credentials",
					"message": "invalid username or password",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    "internal_error",
				"message": "login failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":      result.Token,
			"expires_at": result.Session.ExpiresAt,
			"user": gin.H{
				"id":         result.User.ID,
				"username":   result.User.Username,
				"created_at": result.User.CreatedAt,
				"updated_at": result.User.UpdatedAt,
			},
			"session": gin.H{
				"id":           result.Session.ID,
				"expires_at":   result.Session.ExpiresAt,
				"created_at":   result.Session.CreatedAt,
				"last_seen_at": result.Session.LastSeenAt,
			},
		})
	})

	protected := group.Group("")
	protected.Use(auth.Middleware(service))
	protected.GET("/me", func(c *gin.Context) {
		current, ok := auth.Current(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthorized",
				"message": "unauthorized",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":         current.User.ID,
				"username":   current.User.Username,
				"created_at": current.User.CreatedAt,
				"updated_at": current.User.UpdatedAt,
			},
			"session": gin.H{
				"id":           current.Session.ID,
				"expires_at":   current.Session.ExpiresAt,
				"created_at":   current.Session.CreatedAt,
				"last_seen_at": current.Session.LastSeenAt,
			},
		})
	})
	protected.POST("/logout", func(c *gin.Context) {
		current, ok := auth.Current(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthorized",
				"message": "unauthorized",
			})
			return
		}

		if err := service.Logout(c.Request.Context(), current.Session.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    "internal_error",
				"message": "logout failed",
			})
			return
		}

		c.Status(http.StatusNoContent)
	})
}
