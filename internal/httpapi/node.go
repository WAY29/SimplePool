package httpapi

import (
	"errors"
	"net/http"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/gin-gonic/gin"
)

type nodeUpsertRequest struct {
	Name           string `json:"name" binding:"required"`
	Protocol       string `json:"protocol" binding:"required"`
	Server         string `json:"server" binding:"required"`
	ServerPort     int    `json:"server_port" binding:"required"`
	Enabled        bool   `json:"enabled"`
	TransportJSON  string `json:"transport_json"`
	TLSJSON        string `json:"tls_json"`
	RawPayloadJSON string `json:"raw_payload_json"`
	Credential     string `json:"credential" binding:"required"`
}

type nodeImportRequest struct {
	Payload string `json:"payload" binding:"required"`
}

type nodeProbeRequest struct {
	Force bool     `json:"force"`
	IDs   []string `json:"ids"`
}

type nodeEnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

func registerNodeRoutes(engine *gin.Engine, authService *auth.Service, service *node.Service) {
	group := engine.Group("/api/nodes")
	group.Use(auth.Middleware(authService))
	// @Summary 列出节点
	// @Tags nodes
	// @Produce json
	// @Security BearerAuth
	// @Success 200 {array} node.View
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/nodes [get]
	group.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list nodes failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
	// @Summary 创建手动节点
	// @Tags nodes
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param request body nodeUpsertRequest true "节点请求"
	// @Success 201 {object} node.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/nodes [post]
	group.POST("", func(c *gin.Context) {
		var request nodeUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid node payload")
			return
		}
		item, err := service.CreateManual(c.Request.Context(), node.CreateManualInput{
			Name:           request.Name,
			Protocol:       request.Protocol,
			Server:         request.Server,
			ServerPort:     request.ServerPort,
			TransportJSON:  request.TransportJSON,
			TLSJSON:        request.TLSJSON,
			RawPayloadJSON: request.RawPayloadJSON,
			Credential:     []byte(request.Credential),
		})
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, item)
	})
	// @Summary 获取节点
	// @Tags nodes
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "节点 ID"
	// @Success 200 {object} node.View
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/nodes/{id} [get]
	group.GET("/:id", func(c *gin.Context) {
		item, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 更新节点
	// @Tags nodes
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "节点 ID"
	// @Param request body nodeUpsertRequest true "节点请求"
	// @Success 200 {object} node.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/nodes/{id} [put]
	group.PUT("/:id", func(c *gin.Context) {
		var request nodeUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid node payload")
			return
		}
		item, err := service.Update(c.Request.Context(), c.Param("id"), node.UpdateInput{
			Name:           request.Name,
			Protocol:       request.Protocol,
			Server:         request.Server,
			ServerPort:     request.ServerPort,
			Enabled:        request.Enabled,
			TransportJSON:  request.TransportJSON,
			TLSJSON:        request.TLSJSON,
			RawPayloadJSON: request.RawPayloadJSON,
			Credential:     []byte(request.Credential),
		})
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 设置节点启用状态
	// @Tags nodes
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "节点 ID"
	// @Param request body nodeEnabledRequest true "启用状态"
	// @Success 200 {object} node.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/nodes/{id}/enabled [put]
	group.PUT("/:id/enabled", func(c *gin.Context) {
		var request nodeEnabledRequest
		if err := c.ShouldBindJSON(&request); err != nil || request.Enabled == nil {
			badRequest(c, "invalid node enabled payload")
			return
		}
		item, err := service.SetEnabled(c.Request.Context(), c.Param("id"), *request.Enabled)
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 删除节点
	// @Tags nodes
	// @Security BearerAuth
	// @Param id path string true "节点 ID"
	// @Success 204
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/nodes/{id} [delete]
	group.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleNodeError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	// @Summary 导入节点
	// @Tags nodes
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param request body nodeImportRequest true "导入内容"
	// @Success 201 {array} node.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/nodes/import [post]
	group.POST("/import", func(c *gin.Context) {
		var request nodeImportRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid import payload")
			return
		}
		items, err := service.Import(c.Request.Context(), node.ImportInput{Payload: request.Payload})
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, items)
	})
	// @Summary 探测单个节点
	// @Tags nodes
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "节点 ID"
	// @Param request body nodeProbeRequest false "探测参数"
	// @Success 200 {object} node.ProbeResult
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Failure 503 {object} errorResponse
	// @Router /api/nodes/{id}/probe [post]
	group.POST("/:id/probe", func(c *gin.Context) {
		var request nodeProbeRequest
		_ = c.ShouldBindJSON(&request)
		result, err := service.ProbeByID(c.Request.Context(), c.Param("id"), request.Force)
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	})
	// @Summary 批量探测节点
	// @Tags nodes
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param request body nodeProbeRequest true "批量探测参数"
	// @Success 200 {array} node.ProbeBatchResult
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 503 {object} errorResponse
	// @Router /api/nodes/probe [post]
	group.POST("/probe", func(c *gin.Context) {
		var request nodeProbeRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid probe payload")
			return
		}
		result, err := service.ProbeBatch(c.Request.Context(), request.IDs, request.Force)
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	})
}

func handleNodeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "resource not found"})
	case errors.Is(err, node.ErrUnsupportedProtocol), errors.Is(err, node.ErrInvalidPayload):
		badRequest(c, err.Error())
	case errors.Is(err, node.ErrProbeUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "probe_unavailable", "message": err.Error()})
	default:
		internalError(c, err.Error())
	}
}
