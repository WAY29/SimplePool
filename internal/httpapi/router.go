package httpapi

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/httpapi/webui"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/subscription"
	"github.com/WAY29/SimplePool/internal/tunnel"
	"github.com/gin-gonic/gin"
)

type Options struct {
	AuthService         *auth.Service
	GroupService        *group.Service
	NodeService         *node.Service
	SubscriptionService *subscription.Service
	TunnelService       *tunnel.Service
}

func NewRouter(options Options) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	engine.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	registerAuthRoutes(engine, options.AuthService)
	if options.NodeService != nil {
		registerNodeRoutes(engine, options.AuthService, options.NodeService)
	}
	if options.SubscriptionService != nil {
		registerSubscriptionRoutes(engine, options.AuthService, options.SubscriptionService)
	}
	if options.GroupService != nil {
		registerGroupRoutes(engine, options.AuthService, options.GroupService)
	}
	if options.TunnelService != nil {
		registerTunnelRoutes(engine, options.AuthService, options.TunnelService)
	}

	registerEmbeddedWebUI(engine)

	return engine
}

func registerEmbeddedWebUI(engine *gin.Engine) {
	staticFS := webui.FS()

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		requestPath := c.Request.URL.Path
		if requestPath == "" {
			requestPath = "/"
		}

		if isReservedAPIPath(requestPath) {
			c.Status(http.StatusNotFound)
			return
		}

		target := "index.html"
		cleanPath := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
		if cleanPath != "" && cleanPath != "." {
			if fileExists(staticFS, cleanPath) {
				target = cleanPath
			} else if strings.Contains(path.Base(cleanPath), ".") {
				c.Status(http.StatusNotFound)
				return
			}
		}

		if err := serveEmbeddedFile(c, staticFS, target); err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Abort()
	})
}

func isReservedAPIPath(requestPath string) bool {
	return requestPath == "/api" ||
		strings.HasPrefix(requestPath, "/api/") ||
		requestPath == "/healthz" ||
		strings.HasPrefix(requestPath, "/healthz/") ||
		requestPath == "/readyz" ||
		strings.HasPrefix(requestPath, "/readyz/")
}

func fileExists(staticFS fs.FS, name string) bool {
	info, err := fs.Stat(staticFS, name)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func serveEmbeddedFile(c *gin.Context, staticFS fs.FS, name string) error {
	data, err := fs.ReadFile(staticFS, name)
	if err != nil {
		return err
	}

	http.ServeContent(c.Writer, c.Request, name, time.Time{}, bytes.NewReader(data))
	return nil
}
