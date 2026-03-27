package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/store"
	"github.com/gin-gonic/gin"
)

type groupUpsertRequest struct {
	Name        string `json:"name" binding:"required"`
	FilterRegex string `json:"filter_regex"`
	Description string `json:"description"`
}

func registerGroupRoutes(engine *gin.Engine, authService *auth.Service, service *group.Service) {
	routerGroup := engine.Group("/api/groups")
	routerGroup.Use(auth.Middleware(authService))
	// @Summary 列出节点组
	// @Tags groups
	// @Produce json
	// @Security BearerAuth
	// @Success 200 {array} group.View
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/groups [get]
	routerGroup.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list groups failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
	// @Summary 创建节点组
	// @Tags groups
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param request body groupUpsertRequest true "节点组请求"
	// @Success 201 {object} group.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Router /api/groups [post]
	routerGroup.POST("", func(c *gin.Context) {
		var request groupUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid group payload")
			return
		}
		item, err := service.Create(c.Request.Context(), group.CreateInput{
			Name:        request.Name,
			FilterRegex: request.FilterRegex,
			Description: request.Description,
		})
		if err != nil {
			handleGroupError(c, err)
			return
		}
		c.JSON(http.StatusCreated, item)
	})
	// @Summary 获取节点组
	// @Tags groups
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "节点组 ID"
	// @Success 200 {object} group.View
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/groups/{id} [get]
	routerGroup.GET("/:id", func(c *gin.Context) {
		item, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 更新节点组
	// @Tags groups
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "节点组 ID"
	// @Param request body groupUpsertRequest true "节点组请求"
	// @Success 200 {object} group.View
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/groups/{id} [put]
	routerGroup.PUT("/:id", func(c *gin.Context) {
		var request groupUpsertRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid group payload")
			return
		}
		item, err := service.Update(c.Request.Context(), c.Param("id"), group.UpdateInput{
			Name:        request.Name,
			FilterRegex: request.FilterRegex,
			Description: request.Description,
		})
		if err != nil {
			handleGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
	// @Summary 删除节点组
	// @Tags groups
	// @Security BearerAuth
	// @Param id path string true "节点组 ID"
	// @Success 204
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/groups/{id} [delete]
	routerGroup.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleGroupError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	// @Summary 列出组成员
	// @Tags groups
	// @Produce json
	// @Security BearerAuth
	// @Param id path string true "节点组 ID"
	// @Success 200 {array} group.MemberView
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/groups/{id}/members [get]
	routerGroup.GET("/:id/members", func(c *gin.Context) {
		items, err := service.ListMembers(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, items)
	})
	// @Summary 订阅组成员更新流
	// @Tags groups
	// @Produce application/x-ndjson
	// @Security BearerAuth
	// @Param id path string true "节点组 ID"
	// @Success 200 {string} string "NDJSON stream"
	// @Failure 401 {object} errorResponse
	// @Failure 404 {object} errorResponse
	// @Router /api/groups/{id}/members/stream [get]
	routerGroup.GET("/:id/members/stream", func(c *gin.Context) {
		updates, unsubscribe, err := service.SubscribeMemberUpdates(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleGroupError(c, err)
			return
		}
		defer unsubscribe()

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			internalError(c, "stream not supported")
			return
		}

		c.Header("Content-Type", "application/x-ndjson")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		flusher.Flush()

		encoder := json.NewEncoder(c.Writer)
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-c.Request.Context().Done():
				return
			case item, ok := <-updates:
				if !ok {
					return
				}
				if err := encoder.Encode(item); err != nil {
					return
				}
				flusher.Flush()
			case <-heartbeat.C:
				if _, err := c.Writer.Write([]byte("\n")); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

func handleGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "resource not found"})
	case errors.Is(err, group.ErrInvalidPayload), errors.Is(err, group.ErrInvalidFilter):
		badRequest(c, err.Error())
	default:
		internalError(c, err.Error())
	}
}
