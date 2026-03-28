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
	// @Summary 列出隧道
	// @Tags tunnels
	// @Produce json
	// @Security BearerAuth
	// @Success 200 {array} tunnel.View
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/tunnels [get]
	group.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list tunnels failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
	// @Summary 创建隧道
	// @Tags tunnels
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param request body tunnelUpsertRequest true "隧道请求"
	// @Success 201 {object} tunnel.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/tunnels [post]
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
	// @Summary 获取隧道
	// @Tags tunnels
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "隧道 ID"
	// @Success 200 {object} tunnel.View
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/tunnels/{id} [get]
	group.GET("/:id", func(c *gin.Context) {
		item, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 更新隧道
	// @Tags tunnels
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "隧道 ID"
	// @Param request body tunnelUpsertRequest true "隧道请求"
	// @Success 200 {object} tunnel.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/tunnels/{id} [put]
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
	// @Summary 删除隧道
	// @Tags tunnels
	// @Security BearerAuth
	// @Param id path string true "隧道 ID"
	// @Success 204
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/tunnels/{id} [delete]
	group.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleTunnelError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	// @Summary 启动隧道
	// @Tags tunnels
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "隧道 ID"
	// @Success 200 {object} tunnel.View
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/tunnels/{id}/start [post]
	group.POST("/:id/start", func(c *gin.Context) {
		item, err := service.Start(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 停止隧道
	// @Tags tunnels
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "隧道 ID"
	// @Success 200 {object} tunnel.View
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/tunnels/{id}/stop [post]
	group.POST("/:id/stop", func(c *gin.Context) {
		item, err := service.Stop(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 刷新隧道
	// @Tags tunnels
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "隧道 ID"
	// @Success 200 {object} tunnel.View
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 409 {object} errorResponse
	// @Router /api/tunnels/{id}/refresh [post]
	group.POST("/:id/refresh", func(c *gin.Context) {
		item, err := service.Refresh(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleTunnelError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 获取隧道事件
	// @Tags tunnels
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "隧道 ID"
	// @Param limit query int false "返回条数"
	// @Success 200 {array} tunnel.EventView
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/tunnels/{id}/events [get]
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
	case errors.Is(err, tunnel.ErrNodeLocked):
		c.JSON(http.StatusConflict, gin.H{"code": "node_locked", "message": err.Error()})
	case errors.Is(err, tunnel.ErrTunnelConflict):
		c.JSON(http.StatusConflict, gin.H{"code": "duplicate_tunnel", "message": err.Error()})
	case errors.Is(err, tunnel.ErrRuntimeConfigNil):
		c.JSON(http.StatusConflict, gin.H{"code": "runtime_config_missing", "message": err.Error()})
	case errors.Is(err, tunnel.ErrTunnelNotRunning):
		c.JSON(http.StatusConflict, gin.H{"code": "tunnel_not_running", "message": err.Error()})
	default:
		internalError(c, err.Error())
	}
}
