package singbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/logging"
	sbbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing-box/include"
	sbLog "github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sbjson "github.com/sagernet/sing/common/json"
)

const (
	selectorTag    = "tunnel-selector"
	httpInboundTag = "http-in"
	directTag      = "system-direct"
	localDNSTag    = "local"
)

var (
	ErrInvalidConfig      = errors.New("singbox: invalid config")
	ErrNoRuntimeNodes     = errors.New("singbox: no runtime nodes")
	ErrAlreadyRunning     = errors.New("singbox: runtime already running")
	ErrUnexpectedResponse = errors.New("singbox: unexpected clash api response")
	ErrSelectorSwitch     = errors.New("singbox: selector switch failed")
)

type RuntimeNode struct {
	ID             string
	Name           string
	Protocol       string
	Server         string
	ServerPort     int
	Credential     []byte
	TransportJSON  string
	TLSJSON        string
	RawPayloadJSON string
}

type ProxyAuth struct {
	Username string
	Password string
}

type RenderInput struct {
	ListenHost       string
	ListenPort       int
	LogLevel         string
	Auth             *ProxyAuth
	ControllerPort   int
	ControllerSecret string
	CacheFilePath    string
	Nodes            []RuntimeNode
	CurrentNodeID    string
	DisableSelector  bool
}

type RuntimeLayout struct {
	RootDir       string
	ConfigPath    string
	CachePath     string
	StdoutLogPath string
	StderrLogPath string
}

type ConfigRenderer struct{}

type Compiler interface {
	Format(config []byte) ([]byte, error)
	Check(config []byte) error
}

type ConfigCompiler struct{}

type PortPair struct {
	ProxyPort      int
	ControllerPort int
}

type PortAllocator struct {
	mu        sync.Mutex
	listeners map[int]net.Listener
}

type ProcessStatus string

const (
	ProcessStatusStopped  ProcessStatus = "stopped"
	ProcessStatusStarting ProcessStatus = "starting"
	ProcessStatusRunning  ProcessStatus = "running"
	ProcessStatusError    ProcessStatus = "error"
)

type BoxInstance interface {
	Start() error
	Close() error
}

type BoxFactory interface {
	New(ctx context.Context, config []byte, writer sbLog.PlatformWriter) (BoxInstance, error)
}

type EmbeddedBoxFactory struct{}

type StartRequest struct {
	Layout RuntimeLayout
	Config []byte
}

type SupervisorOptions struct {
	Compiler Compiler
	Factory  BoxFactory
	Now      func() time.Time
}

type Supervisor struct {
	mu        sync.Mutex
	compiler  Compiler
	factory   BoxFactory
	now       func() time.Time
	status    ProcessStatus
	lastError error
	instance  BoxInstance
	cancel    context.CancelFunc
	logWriter *fileLogWriter
}

type fileLogWriter struct {
	mu     sync.Mutex
	stdout *os.File
	stderr *os.File
	now    func() time.Time
}

type ClashAPIClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

type ProxyInfo struct {
	Type string   `json:"type"`
	Name string   `json:"name"`
	Now  string   `json:"now,omitempty"`
	All  []string `json:"all,omitempty"`
}

func NewRuntimeLayout(runtimeRoot, tunnelID string) RuntimeLayout {
	root := filepath.Join(runtimeRoot, "tunnels", "tunnel-"+tunnelID)
	return RuntimeLayout{
		RootDir:       root,
		ConfigPath:    filepath.Join(root, "config.json"),
		CachePath:     filepath.Join(root, "cache.db"),
		StdoutLogPath: filepath.Join(root, "stdout.log"),
		StderrLogPath: filepath.Join(root, "stderr.log"),
	}
}

func NewConfigRenderer() *ConfigRenderer {
	return &ConfigRenderer{}
}

func (r *ConfigRenderer) Render(input RenderInput) ([]byte, error) {
	if input.ListenPort <= 0 || input.ControllerPort <= 0 || strings.TrimSpace(input.ControllerSecret) == "" {
		return nil, ErrInvalidConfig
	}
	if len(input.Nodes) == 0 {
		return nil, ErrNoRuntimeNodes
	}
	listenHost := strings.TrimSpace(input.ListenHost)
	if listenHost == "" {
		listenHost = "127.0.0.1"
	}

	outbounds := make([]any, 0, len(input.Nodes)+2)
	selectorOutbounds := make([]string, 0, len(input.Nodes))
	currentTag := ""
	for _, node := range input.Nodes {
		tag := outboundTag(node.ID)
		outbound, err := buildRuntimeOutbound(tag, node)
		if err != nil {
			return nil, err
		}
		outbounds = append(outbounds, outbound)
		selectorOutbounds = append(selectorOutbounds, tag)
		if node.ID == input.CurrentNodeID {
			currentTag = tag
		}
	}
	if currentTag == "" {
		currentTag = selectorOutbounds[0]
	}

	if !input.DisableSelector {
		outbounds = append(outbounds, map[string]any{
			"type":      "selector",
			"tag":       selectorTag,
			"outbounds": selectorOutbounds,
			"default":   currentTag,
		})
	}
	outbounds = append(outbounds, map[string]any{
		"type": "direct",
		"tag":  directTag,
	})

	finalOutbound := currentTag
	if !input.DisableSelector {
		finalOutbound = selectorTag
	}

	experimental := map[string]any{
		"clash_api": map[string]any{
			"external_controller": fmt.Sprintf("127.0.0.1:%d", input.ControllerPort),
			"secret":              input.ControllerSecret,
		},
	}
	if input.CacheFilePath != "" {
		experimental["cache_file"] = map[string]any{
			"enabled": true,
			"path":    input.CacheFilePath,
		}
	}

	config := map[string]any{
		"log": map[string]any{
			"level": logging.NormalizeLevel(input.LogLevel),
		},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{
					"type": "local",
					"tag":  localDNSTag,
				},
			},
			"final": localDNSTag,
		},
		"inbounds":  []any{buildHTTPInbound(listenHost, input.ListenPort, input.Auth)},
		"outbounds": outbounds,
		"route": map[string]any{
			"final":                   finalOutbound,
			"default_domain_resolver": localDNSTag,
		},
		"experimental": experimental,
	}
	return json.Marshal(config)
}

func buildHTTPInbound(host string, port int, auth *ProxyAuth) map[string]any {
	inbound := map[string]any{
		"type":        "http",
		"tag":         httpInboundTag,
		"listen":      host,
		"listen_port": port,
	}
	if auth != nil && (auth.Username != "" || auth.Password != "") {
		inbound["users"] = []map[string]any{{
			"username": auth.Username,
			"password": auth.Password,
		}}
	}
	return inbound
}

func outboundTag(nodeID string) string {
	return "node-" + nodeID
}

func serverNeedsResolver(server string) bool {
	server = strings.TrimSpace(server)
	if server == "" {
		return false
	}
	return net.ParseIP(server) == nil
}

func buildRuntimeOutbound(tag string, node RuntimeNode) (map[string]any, error) {
	credential := make(map[string]any)
	if err := json.Unmarshal(node.Credential, &credential); err != nil {
		return nil, err
	}
	transport := decodeJSONMap(node.TransportJSON)
	tlsOptions := decodeJSONMap(node.TLSJSON)
	rawPayload := decodeJSONMap(node.RawPayloadJSON)

	outbound := map[string]any{
		"type":        normalizeProtocol(node.Protocol),
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.ServerPort,
	}
	if serverNeedsResolver(node.Server) {
		outbound["domain_resolver"] = localDNSTag
	}

	switch normalizeProtocol(node.Protocol) {
	case "ss":
		outbound["method"] = stringValue(credential["method"], "")
		outbound["password"] = stringValue(credential["password"], "")
		if plugin := stringValue(rawPayload["plugin"], ""); plugin != "" {
			outbound["plugin"] = plugin
		}
	case "trojan":
		outbound["password"] = stringValue(credential["password"], "")
	case "vmess":
		outbound["uuid"] = stringValue(credential["uuid"], "")
		outbound["security"] = stringValue(rawPayload["scy"], "auto")
		if alterID, err := intValue(rawPayload["aid"]); err == nil && alterID > 0 {
			outbound["alter_id"] = alterID
		}
	case "vless":
		outbound["uuid"] = stringValue(credential["uuid"], "")
		if flow := stringValue(rawPayload["flow"], ""); flow != "" {
			outbound["flow"] = flow
		}
	case "hysteria2":
		outbound["password"] = stringValue(credential["password"], "")
		if up, err := intValue(rawPayload["up_mbps"]); err == nil && up > 0 {
			outbound["up_mbps"] = up
		}
		if down, err := intValue(rawPayload["down_mbps"]); err == nil && down > 0 {
			outbound["down_mbps"] = down
		}
		if obfs := stringValue(transport["obfs"], ""); obfs != "" {
			outbound["obfs"] = map[string]any{
				"type":     obfs,
				"password": stringValue(transport["obfs_password"], ""),
			}
		}
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", node.Protocol)
	}

	if transportMap := buildTransportMap(transport); len(transportMap) > 0 {
		outbound["transport"] = transportMap
	}
	if tlsMap := buildTLSMap(tlsOptions); len(tlsMap) > 0 {
		outbound["tls"] = tlsMap
	}

	return outbound, nil
}

func (c *ConfigCompiler) Format(config []byte) ([]byte, error) {
	formatted, err := libbox.FormatConfig(string(config))
	if err != nil {
		return nil, err
	}
	return []byte(formatted.Value), nil
}

func (c *ConfigCompiler) Check(config []byte) error {
	return libbox.CheckConfig(string(config))
}

func WriteAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".simplepool-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	success = true
	return nil
}

func NewPortAllocator() *PortAllocator {
	return &PortAllocator{
		listeners: make(map[int]net.Listener),
	}
}

func (a *PortAllocator) Allocate() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	a.mu.Lock()
	a.listeners[port] = listener
	a.mu.Unlock()
	return port, nil
}

func (a *PortAllocator) AllocatePair() (PortPair, error) {
	proxyPort, err := a.Allocate()
	if err != nil {
		return PortPair{}, err
	}
	controllerPort, err := a.Allocate()
	if err != nil {
		_ = a.Release(proxyPort)
		return PortPair{}, err
	}
	return PortPair{
		ProxyPort:      proxyPort,
		ControllerPort: controllerPort,
	}, nil
}

func (a *PortAllocator) Release(port int) error {
	a.mu.Lock()
	listener := a.listeners[port]
	delete(a.listeners, port)
	a.mu.Unlock()
	if listener == nil {
		return nil
	}
	return listener.Close()
}

func (a *PortAllocator) Close() error {
	a.mu.Lock()
	ports := make([]int, 0, len(a.listeners))
	for port := range a.listeners {
		ports = append(ports, port)
	}
	a.mu.Unlock()

	var errs []error
	for _, port := range ports {
		if err := a.Release(port); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func NewSupervisor(options SupervisorOptions) *Supervisor {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	if options.Compiler == nil {
		options.Compiler = &ConfigCompiler{}
	}
	if options.Factory == nil {
		options.Factory = &EmbeddedBoxFactory{}
	}
	return &Supervisor{
		compiler: options.Compiler,
		factory:  options.Factory,
		now:      now,
		status:   ProcessStatusStopped,
	}
}

func (s *Supervisor) Start(ctx context.Context, request StartRequest) error {
	s.mu.Lock()
	if s.instance != nil {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.status = ProcessStatusStarting
	s.lastError = nil
	s.mu.Unlock()

	formatted, err := s.compiler.Format(request.Config)
	if err != nil {
		s.fail(err)
		return err
	}
	if err := s.compiler.Check(formatted); err != nil {
		s.fail(err)
		return err
	}
	if err := os.MkdirAll(request.Layout.RootDir, 0o755); err != nil {
		s.fail(err)
		return err
	}

	writer, err := newFileLogWriter(request.Layout, s.now)
	if err != nil {
		s.fail(err)
		return err
	}

	runtimeParent := context.Background()
	if ctx != nil {
		runtimeParent = context.WithoutCancel(ctx)
	}
	runtimeCtx, cancel := context.WithCancel(runtimeParent)
	instance, err := s.factory.New(runtimeCtx, formatted, writer)
	if err != nil {
		cancel()
		_ = writer.Close()
		s.fail(err)
		return err
	}
	if err := instance.Start(); err != nil {
		cancel()
		_ = instance.Close()
		_ = writer.Close()
		s.fail(err)
		return err
	}

	s.mu.Lock()
	s.instance = instance
	s.cancel = cancel
	s.logWriter = writer
	s.status = ProcessStatusRunning
	s.lastError = nil
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) Stop() error {
	s.mu.Lock()
	instance := s.instance
	cancel := s.cancel
	writer := s.logWriter
	s.instance = nil
	s.cancel = nil
	s.logWriter = nil
	s.status = ProcessStatusStopped
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var errs []error
	if instance != nil {
		if err := instance.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if writer != nil {
		if err := writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		err := errors.Join(errs...)
		s.fail(err)
		return err
	}
	return nil
}

func (s *Supervisor) Status() ProcessStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Supervisor) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

func (s *Supervisor) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = ProcessStatusError
	s.lastError = err
}

func newFileLogWriter(layout RuntimeLayout, now func() time.Time) (*fileLogWriter, error) {
	if err := os.MkdirAll(layout.RootDir, 0o755); err != nil {
		return nil, err
	}
	stdout, err := os.OpenFile(layout.StdoutLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	stderr, err := os.OpenFile(layout.StderrLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}
	return &fileLogWriter{
		stdout: stdout,
		stderr: stderr,
		now:    now,
	}, nil
}

func (w *fileLogWriter) WriteMessage(level sbLog.Level, message string) {
	if w == nil {
		return
	}
	target := w.stdout
	if level <= sbLog.LevelWarn {
		target = w.stderr
	}
	line := fmt.Sprintf("%s [%s] %s\n", w.now().UTC().Format(time.RFC3339Nano), sbLog.FormatLevel(level), message)
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = target.WriteString(line)
}

func (w *fileLogWriter) Close() error {
	if w == nil {
		return nil
	}
	var errs []error
	if w.stdout != nil {
		if err := w.stdout.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if w.stderr != nil {
		if err := w.stderr.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (f *EmbeddedBoxFactory) New(ctx context.Context, config []byte, writer sbLog.PlatformWriter) (BoxInstance, error) {
	singCtx := include.Context(ctx)
	options, err := sbjson.UnmarshalExtendedContext[option.Options](singCtx, config)
	if err != nil {
		return nil, err
	}
	return sbbox.New(sbbox.Options{
		Context:           singCtx,
		Options:           options,
		PlatformLogWriter: writer,
	})
}

func NewClashAPIClient(baseURL, secret string, client *http.Client) *ClashAPIClient {
	baseURL = strings.TrimRight(baseURL, "/")
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &ClashAPIClient{
		baseURL: baseURL,
		secret:  secret,
		client:  client,
	}
}

func (c *ClashAPIClient) GetProxies(ctx context.Context) (map[string]ProxyInfo, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/proxies", nil)
	if err != nil {
		return nil, err
	}
	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, parseClashError(response, ErrUnexpectedResponse)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Proxies map[string]ProxyInfo `json:"proxies"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Proxies != nil {
		return payload.Proxies, nil
	}

	var raw map[string]ProxyInfo
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *ClashAPIClient) GetProxy(ctx context.Context, tag string) (*ProxyInfo, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/proxies/"+url.PathEscape(tag), nil)
	if err != nil {
		return nil, err
	}
	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, parseClashError(response, ErrUnexpectedResponse)
	}
	var info ProxyInfo
	if err := readJSONBody(response, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *ClashAPIClient) SwitchSelector(ctx context.Context, selector, outbound string) error {
	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(map[string]string{"name": outbound}); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/proxies/"+url.PathEscape(selector), body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return parseClashError(response, ErrSelectorSwitch)
	}
	return nil
}

func (c *ClashAPIClient) do(request *http.Request) (*http.Response, error) {
	if c.secret != "" {
		request.Header.Set("Authorization", "Bearer "+c.secret)
	}
	return c.client.Do(request)
}

func parseClashError(response *http.Response, cause error) error {
	var payload struct {
		Message string `json:"message"`
	}
	_ = readJSONBody(response, &payload)
	if payload.Message != "" {
		return fmt.Errorf("%w: %s", cause, payload.Message)
	}
	return fmt.Errorf("%w: %s", cause, response.Status)
}

func readJSONBody(response *http.Response, target any) error {
	return json.NewDecoder(response.Body).Decode(target)
}

func MapRuntimeError(err error) string {
	switch {
	case err == nil:
		return domain.TunnelStatusRunning
	case errors.Is(err, context.Canceled):
		return domain.TunnelStatusStopped
	case errors.Is(err, ErrSelectorSwitch), errors.Is(err, ErrUnexpectedResponse):
		return domain.TunnelStatusDegraded
	default:
		return domain.TunnelStatusError
	}
}
