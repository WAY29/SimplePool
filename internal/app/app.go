package app

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/WAY29/SimplePool/internal/apperr"
	"github.com/WAY29/SimplePool/internal/auth"
	"github.com/WAY29/SimplePool/internal/config"
	"github.com/WAY29/SimplePool/internal/crypto"
	"github.com/WAY29/SimplePool/internal/group"
	"github.com/WAY29/SimplePool/internal/httpapi"
	"github.com/WAY29/SimplePool/internal/logging"
	"github.com/WAY29/SimplePool/internal/node"
	"github.com/WAY29/SimplePool/internal/runtime/singbox"
	"github.com/WAY29/SimplePool/internal/settings"
	"github.com/WAY29/SimplePool/internal/store/sqlite"
	"github.com/WAY29/SimplePool/internal/subscription"
	"github.com/WAY29/SimplePool/internal/tunnel"
	"log/slog"
)

type App struct {
	cfg      config.Config
	logger   *slog.Logger
	db       *sql.DB
	server   *http.Server
	listener net.Listener
	tunnels  *tunnel.Service
	mu       sync.Mutex
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	return NewWithDependencies(ctx, cfg, Dependencies{})
}

type Dependencies struct {
	Now                 func() time.Time
	NodeProber          node.Prober
	SubscriptionFetcher subscription.Fetcher
	TunnelRuntime       tunnel.RuntimeManager
}

func NewWithDependencies(ctx context.Context, cfg config.Config, deps Dependencies) (*App, error) {
	const op = "app.New"
	const probeCacheTTL = 5 * time.Minute

	if err := ensureDirectories(cfg.Paths); err != nil {
		return nil, apperr.Wrap(apperr.CodeRuntime, op, err)
	}

	logger := logging.New(cfg.LogLevel)

	db, err := sqlite.Open(ctx, cfg.Paths.DBPath)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeStore, op, err)
	}

	if err := sqlite.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, apperr.Wrap(apperr.CodeStore, op, err)
	}

	repos := sqlite.NewRepositories(db)
	cipher, err := crypto.NewAESGCM(cfg.Security.MasterKey)
	if err != nil {
		_ = db.Close()
		return nil, apperr.Wrap(apperr.CodeSecurity, op, err)
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	settingsService := settings.NewService(settings.Options{
		AppSettings: repos.AppSettings,
		Now:         now,
	})
	prober := deps.NodeProber
	if prober == nil {
		prober = singbox.NewDynamicProber(func(ctx context.Context) string {
			view, err := settingsService.GetProbeConfig(ctx)
			if err != nil {
				logger.Warn("load probe settings failed, use default test url", "error", err)
				return settings.DefaultProbeTestURL
			}
			return view.TestURL
		}, 3*time.Second, cfg.LogLevel, cfg.UpstreamHTTPProxyURL)
	}
	fetcher := deps.SubscriptionFetcher
	if fetcher == nil {
		fetcher = subscription.NewHTTPFetcher(10 * time.Second)
	}
	runtimeManager := deps.TunnelRuntime
	if runtimeManager == nil {
		runtimeManager = tunnel.NewRuntimeManager(tunnel.RuntimeManagerOptions{Now: now})
	}
	nodeService := node.NewService(node.Options{
		Nodes:          repos.Nodes,
		LatencySamples: repos.LatencySamples,
		Cipher:         cipher,
		Prober:         prober,
		Now:            now,
		ProbeCacheTTL:  probeCacheTTL,
	})
	subscriptionService := subscription.NewService(subscription.Options{
		SubscriptionSources: repos.SubscriptionSources,
		Nodes:               repos.Nodes,
		LatencySamples:      repos.LatencySamples,
		Cipher:              cipher,
		Fetcher:             fetcher,
		Prober:              prober,
		Now:                 now,
		ProbeCacheTTL:       probeCacheTTL,
	})
	groupService := group.NewService(group.Options{
		Groups: repos.Groups,
		Nodes:  repos.Nodes,
		Now:    now,
	})
	authService := auth.NewService(auth.Options{
		AdminUsers: repos.AdminUsers,
		Sessions:   repos.Sessions,
		Now:        now,
		SessionTTL: 7 * 24 * time.Hour,
	})
	if err := authService.EnsureAdmin(ctx, cfg.Admin.Username, cfg.Admin.Password); err != nil {
		_ = db.Close()
		return nil, apperr.Wrap(apperr.CodeRuntime, op, err)
	}
	tunnelService := tunnel.NewService(tunnel.Options{
		Tunnels:              repos.Tunnels,
		TunnelEvents:         repos.TunnelEvents,
		LatencySamples:       repos.LatencySamples,
		Groups:               groupService,
		Nodes:                repos.Nodes,
		LogLevel:             cfg.LogLevel,
		Cipher:               cipher,
		Prober:               prober,
		ProbeCacheTTL:        probeCacheTTL,
		Runtime:              runtimeManager,
		RuntimeRoot:          cfg.Paths.RuntimeDir,
		UpstreamHTTPProxyURL: cfg.UpstreamHTTPProxyURL,
		Now:                  now,
		Logger:               logger,
	})
	if err := tunnelService.Initialize(ctx); err != nil {
		_ = tunnelService.Close()
		_ = db.Close()
		return nil, apperr.Wrap(apperr.CodeRuntime, op, err)
	}

	engine := httpapi.NewRouter(httpapi.Options{
		AuthService:         authService,
		Debug:               cfg.Debug,
		GroupService:        groupService,
		NodeService:         nodeService,
		SettingsService:     settingsService,
		SubscriptionService: subscriptionService,
		TunnelService:       tunnelService,
	})

	return &App{
		cfg:     cfg,
		logger:  logger,
		db:      db,
		tunnels: tunnelService,
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

func (a *App) Start() error {
	const op = "app.Start"

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.listener != nil {
		return nil
	}

	listener, err := net.Listen("tcp", a.cfg.HTTPAddr)
	if err != nil {
		return apperr.Wrap(apperr.CodeRuntime, op, err)
	}

	a.listener = listener

	go func() {
		if err := a.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("http server stopped unexpectedly", "error", err)
		}
	}()

	a.logger.Info("simplepool api started", "addr", listener.Addr().String(), "db", a.cfg.Paths.DBPath)
	return nil
}

func (a *App) Address() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.listener == nil {
		return a.cfg.HTTPAddr
	}

	return a.listener.Addr().String()
}

func (a *App) Shutdown(ctx context.Context) error {
	const op = "app.Shutdown"

	a.mu.Lock()
	server := a.server
	listener := a.listener
	db := a.db
	tunnels := a.tunnels
	a.listener = nil
	a.mu.Unlock()

	var errs []error

	if server != nil {
		if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, apperr.Wrap(apperr.CodeRuntime, op, err))
		}
	}

	if listener != nil {
		_ = listener.Close()
	}

	if db != nil {
		if err := db.Close(); err != nil {
			errs = append(errs, apperr.Wrap(apperr.CodeStore, op, err))
		}
	}
	if tunnels != nil {
		if err := tunnels.Close(); err != nil {
			errs = append(errs, apperr.Wrap(apperr.CodeRuntime, op, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func ensureDirectories(paths config.Paths) error {
	for _, dir := range []string{paths.DataDir, paths.RuntimeDir, paths.TempDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}
