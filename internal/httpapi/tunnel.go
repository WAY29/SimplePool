package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/WAY29/SimplePool/internal/tunnel"
	"github.com/gin-gonic/gin"
)

type tunnelUpsertRequest struct {
	Name       string `json:"name" binding:"required"`
	GroupID    string `json:"group_id" binding:"required"`
	ListenHost string `json:"listen_host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

func registerTunnelRoutes(engine *gin.Engine, authService *auth.Service, service *tunnel.Service) {
	group := engine.Group("/api/tunnels")
	group.Use(auth.Middleware(authService))
	group.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list tunnels failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
	group.POST("", func(c *gin.Context) {
		var request tunnelUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid tunnel payload")
			return
		}
		item, err := service.Create(c.Request.Context(), tunnel.CreateInput{
			Name:       request.Name,
			GroupID:    request.GroupID,
			ListenHost: request.ListenHost,
			Username:   request.Username,
			Password:   request.Password,
		})
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusCreated, item)
	})
	group.GET("/:id", func(c *gin.Context) {
		item, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	group.PUT("/:id", func(c *gin.Context) {
		var request tunnelUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid tunnel payload")
			return
		}
		item, err := service.Update(c.Request.Context(), c.Param("id"), tunnel.UpdateInput{
			Name:       request.Name,
			GroupID:    request.GroupID,
			ListenHost: request.ListenHost,
			Username:   request.Username,
			Password:   request.Password,
		})
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	group.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleTunnelError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	group.POST("/:id/start", func(c *gin.Context) {
		item, err := service.Start(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	group.POST("/:id/stop", func(c *gin.Context) {
		item, err := service.Stop(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	group.POST("/:id/refresh", func(c *gin.Context) {
		item, err := service.Refresh(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	group.GET("/:id/events", func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		items, err := service.ListEvents(c.Request.Context(), c.Param("id"), limit)
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, items)
	})
}

func handleTunnelError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "resource not found"})
	case errors.Is(err, tunnel.ErrInvalidPayload):
		badRequest(c, err.Error())
	case errors.Is(err, tunnel.ErrNoAvailableNodes):
		c.JSON(http.StatusConflict, gin.H{"code": "no_available_nodes", "message": err.Error()})
	case errors.Is(err, tunnel.ErrTunnelNotRunning):
		c.JSON(http.StatusConflict, gin.H{"code": "tunnel_not_running", "message": err.Error()})
	default:
		internalError(c, err.Error())
	}
}
