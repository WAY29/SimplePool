package httpapi

import (
	"errors"
	"net/http"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/settings"
	"github.com/gin-gonic/gin"
)

type probeSettingsUpdateRequest struct {
	TestURL string `json:"test_url" binding:"required"`
}

func registerSettingsRoutes(engine *gin.Engine, authService *auth.Service, service *settings.Service) {
	group := engine.Group("/api/settings")
	group.Use(auth.Middleware(authService))

	// @Summary 获取测速设置
	// @Tags settings
	// @Produce json
	// @Security BearerAuth
	// @Success 200 {object} settings.ProbeConfigView
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/settings/probe [get]
	group.GET("/probe", func(c *gin.Context) {
		view, err := service.GetProbeConfig(c.Request.Context())
		if err != nil {
			internalError(c, "get probe settings failed")
			return
		}
		c.JSON(http.StatusOK, view)
	})

	// @Summary 更新测速设置
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Security BearerAuth
	// @Param request body probeSettingsUpdateRequest true "测速设置"
	// @Success 200 {object} settings.ProbeConfigView
	// @Failure 400 {object} errorResponse
	// @Failure 401 {object} errorResponse
	// @Failure 500 {object} errorResponse
	// @Router /api/settings/probe [put]
	group.PUT("/probe", func(c *gin.Context) {
		var request probeSettingsUpdateRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			badRequest(c, "invalid probe settings payload")
			return
		}
		view, err := service.SetProbeTestURL(c.Request.Context(), request.TestURL)
		if err != nil {
			handleSettingsError(c, err)
			return
		}
		c.JSON(http.StatusOK, view)
	})
}

func handleSettingsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, settings.ErrInvalidProbeTestURL):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_probe_url", "message": err.Error()})
	default:
		internalError(c, err.Error())
	}
}
