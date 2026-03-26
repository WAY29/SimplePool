package httpapi

import (
	"errors"
	"net/http"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/WAY29/SimplePool/internal/subscription"
	"github.com/gin-gonic/gin"
)

type subscriptionUpsertRequest struct {
	Name    string `json:"name" binding:"required"`
	URL     string `json:"url" binding:"required"`
	Enabled bool   `json:"enabled"`
}

type subscriptionRefreshRequest struct {
	Force bool `json:"force"`
}

func registerSubscriptionRoutes(engine *gin.Engine, authService *auth.Service, service *subscription.Service) {
	group := engine.Group("/api/subscriptions")
	group.Use(auth.Middleware(authService))
	group.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list subscriptions failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
	group.POST("", func(c *gin.Context) {
		var request subscriptionUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid subscription payload")
			return
		}
		item, err := service.Create(c.Request.Context(), subscription.CreateInput{
			Name: request.Name,
			URL:  request.URL,
		})
		if err != nil {
			handleSubscriptionError(c, err)
			return
		}
		c.JSON(http.StatusCreated, item)
	})
	group.PUT("/:id", func(c *gin.Context) {
		var request subscriptionUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid subscription payload")
			return
		}
		item, err := service.Update(c.Request.Context(), c.Param("id"), subscription.UpdateInput{
			Name:    request.Name,
			URL:     request.URL,
			Enabled: request.Enabled,
		})
		if err != nil {
			handleSubscriptionError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	group.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleSubscriptionError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	group.POST("/:id/refresh", func(c *gin.Context) {
		var request subscriptionRefreshRequest
		_ = c.ShouldBindJSON(&request)
		result, err := service.Refresh(c.Request.Context(), c.Param("id"), request.Force)
		if err != nil {
			handleSubscriptionError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	})
}

func handleSubscriptionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "resource not found"})
	case errors.Is(err, subscription.ErrDuplicateSource):
		c.JSON(http.StatusConflict, gin.H{"code": "duplicate_source", "message": err.Error()})
	case errors.Is(err, subscription.ErrInvalidURL), errors.Is(err, subscription.ErrFetchFailed):
		badRequest(c, err.Error())
	default:
		internalError(c, err.Error())
	}
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": message})
}

func internalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": message})
}
