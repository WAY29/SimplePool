package httpapi

import (
	"errors"
	"net/http"

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
	routerGroup.GET("", func(c *gin.Context) {
		items, err := service.List(c.Request.Context())
		if err != nil {
			internalError(c, "list groups failed")
			return
		}
		c.JSON(http.StatusOK, items)
	})
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
	routerGroup.GET("/:id", func(c *gin.Context) {
		item, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	})
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
	routerGroup.DELETE("/:id", func(c *gin.Context) {
		if err := service.Delete(c.Request.Context(), c.Param("id")); err != nil {
			handleGroupError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	routerGroup.GET("/:id/members", func(c *gin.Context) {
		items, err := service.ListMembers(c.Request.Context(), c.Param("id"))
		if err != nil {
			handleGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, items)
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
