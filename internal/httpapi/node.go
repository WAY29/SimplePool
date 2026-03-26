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

func registerNodeRoutes(engine *gin.Engine, authService *auth.Service, service *node.Service) {
	group := engine.Group("/api/nodes")
	group.Use(auth.Middleware(authService))
	group.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list nodes failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
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
	group.GET("/:id", func(c *gin.Context) {
		item, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleNodeError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
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
	group.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleNodeError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
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
