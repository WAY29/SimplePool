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
	// @Summary 列出订阅源
	// @Tags subscriptions
	// @Produce json
	// @Security BearerAuth
	// @Success 200 {array} subscription.View
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/subscriptions [get]
	group.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list subscriptions failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
	// @Summary 创建订阅源
	// @Tags subscriptions
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param request body subscriptionUpsertRequest true "订阅请求"
	// @Success 201 {object} subscription.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/subscriptions [post]
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
	// @Summary 更新订阅源
	// @Tags subscriptions
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "订阅 ID"
	// @Param request body subscriptionUpsertRequest true "订阅请求"
	// @Success 200 {object} subscription.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/subscriptions/{id} [put]
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
	// @Summary 删除订阅源
	// @Tags subscriptions
	// @Security BearerAuth
	// @Param id path string true "订阅 ID"
	// @Success 204
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/subscriptions/{id} [delete]
	group.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleSubscriptionError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	// @Summary 刷新订阅源
	// @Tags subscriptions
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "订阅 ID"
	// @Param request body subscriptionRefreshRequest false "刷新参数"
	// @Success 200 {object} subscription.RefreshResult
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/subscriptions/{id}/refresh [post]
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
