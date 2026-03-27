package httpapi

import (
	"net/http"

	"github.com/WAY29/SimplePool/internal/httpapi/openapi"
	"github.com/gin-gonic/gin"
)

func registerOpenAPIRoute(engine *gin.Engine) {
	engine.GET("/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", openapi.JSON())
	})
}
