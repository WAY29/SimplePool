package singbox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WAY29/SimplePool/internal/domain"
	"github.com/WAY29/SimplePool/internal/runtime/singbox"
	sbLog "github.com/sagernet/sing-box/log"
)

func TestConfigRendererRendersHTTPInboundSelectorAndClashAPI(t *testing.T) {
	renderer := singbox.NewConfigRenderer()

	configJSON, err := renderer.Render(singbox.RenderInput{
		ListenHost:       "127.0.0.1",
		ListenPort:       18080,
		LogLevel:         "warning",
		ControllerPort:   19090,
		ControllerSecret: "secret-1",
		CurrentNodeID:    "2",
		Auth: &singbox.ProxyAuth{
			Username: "user-1",
			Password: "pass-1",
		},
		Nodes: []singbox.RuntimeNode{
			{
				ID:             "1",
				Name:           "HK-A",
				Protocol:       "vmess",
				Server:         "1.1.1.1",
				ServerPort:     443,
				Credential:     []byte(`{"uuid":"u-1"}`),
				TransportJSON:  `{"type":"ws","path":"/ws","host":"hk.example.com"}`,
				TLSJSON:        `{"enabled":true,"server_name":"hk.example.com"}`,
				RawPayloadJSON: `{"scy":"auto"}`,
			},
			{
				ID:             "2",
				Name:           "JP-A",
				Protocol:       "trojan",
				Server:         "2.2.2.2",
				ServerPort:     443,
				Credential:     []byte(`{"password":"secret"}`),
				TransportJSON:  `{}`,
				TLSJSON:        `{"enabled":true,"server_name":"jp.example.com"}`,
				RawPayloadJSON: `{}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(configJSON, &config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	inbounds := config["inbounds"].([]any)
	firstInbound := inbounds[0].(map[string]any)
	if firstInbound["type"] != "http" || int(firstInbound["listen_port"].(float64)) != 18080 {
		t.Fatalf("http inbound = %+v, want http:18080", firstInbound)
	}
	users := firstInbound["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("len(users) = %d, want 1", len(users))
	}

	outbounds := config["outbounds"].([]any)
	var selector map[string]any
	var directFound bool
	for _, item := range outbounds {
		entry := item.(map[string]any)
		switch entry["tag"] {
		case "tunnel-selector":
			selector = entry
		case "system-direct":
			directFound = true
		}
	}
	if selector == nil {
		t.Fatal("selector outbound missing")
	}
	if selector["default"] != "node-2" {
		t.Fatalf("selector default = %v, want node-2", selector["default"])
	}
	if !directFound {
		t.Fatal("direct outbound missing")
	}

	experimental := config["experimental"].(map[string]any)
	clashAPI := experimental["clash_api"].(map[string]any)
	if clashAPI["external_controller"] != "127.0.0.1:19090" {
		t.Fatalf("external_controller = %v, want 127.0.0.1:19090", clashAPI["external_controller"])
	}
	if clashAPI["secret"] != "secret-1" {
		t.Fatalf("secret = %v, want secret-1", clashAPI["secret"])
	}
	if _, exists := experimental["cache_file"]; exists {
		t.Fatalf("experimental.cache_file should be omitted, got %+v", experimental["cache_file"])
	}
	logConfig := config["log"].(map[string]any)
	if logConfig["level"] != "warn" {
		t.Fatalf("log.level = %v, want warn", logConfig["level"])
	}
}

func TestConfigRendererDefaultsLogLevelToInfo(t *testing.T) {
	renderer := singbox.NewConfigRenderer()

	configJSON, err := renderer.Render(singbox.RenderInput{
		ListenHost:       "127.0.0.1",
		ListenPort:       18080,
		ControllerPort:   19090,
		ControllerSecret: "secret-1",
		Nodes: []singbox.RuntimeNode{{
			ID:             "1",
			Name:           "HK-A",
			Protocol:       "trojan",
			Server:         "1.1.1.1",
			ServerPort:     443,
			Credential:     []byte(`{"password":"secret"}`),
			TransportJSON:  `{}`,
			TLSJSON:        `{"enabled":true,"server_name":"hk.example.com"}`,
			RawPayloadJSON: `{}`,
		}},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(configJSON, &config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	logConfig := config["log"].(map[string]any)
	if logConfig["level"] != "info" {
		t.Fatalf("log.level = %v, want info", logConfig["level"])
	}
}

func TestConfigRendererUsesLocalDNSResolverForDomainOutbounds(t *testing.T) {
	renderer := singbox.NewConfigRenderer()
	compiler := &singbox.ConfigCompiler{}

	configJSON, err := renderer.Render(singbox.RenderInput{
		ListenHost:       "127.0.0.1",
		ListenPort:       18080,
		ControllerPort:   19090,
		ControllerSecret: "secret-1",
		CurrentNodeID:    "1",
		Nodes: []singbox.RuntimeNode{{
			ID:             "1",
			Name:           "Domain-VLESS",
			Protocol:       "vless",
			Server:         "downloadcfpro.example.com",
			ServerPort:     443,
			Credential:     []byte(`{"uuid":"u-1"}`),
			TransportJSON:  `{"type":"tcp"}`,
			TLSJSON:        `{"enabled":true,"server_name":"downloadcfpro.example.com"}`,
			RawPayloadJSON: `{}`,
		}},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if err := compiler.Check(configJSON); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(configJSON, &config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	dnsConfig, ok := config["dns"].(map[string]any)
	if !ok {
		t.Fatalf("dns config missing: %+v", config)
	}
	servers, ok := dnsConfig["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("dns.servers = %+v, want single local server", dnsConfig["servers"])
	}
	server := servers[0].(map[string]any)
	if server["tag"] != "local" || server["type"] != "local" {
		t.Fatalf("dns server = %+v, want local/local", server)
	}
	if _, exists := server["detour"]; exists {
		t.Fatalf("dns server detour should be omitted, got %+v", server)
	}

	routeConfig := config["route"].(map[string]any)
	if routeConfig["default_domain_resolver"] != "local" {
		t.Fatalf("route.default_domain_resolver = %v, want local", routeConfig["default_domain_resolver"])
	}

	outbounds := config["outbounds"].([]any)
	firstOutbound := outbounds[0].(map[string]any)
	if firstOutbound["domain_resolver"] != "local" {
		t.Fatalf("outbound.domain_resolver = %v, want local", firstOutbound["domain_resolver"])
	}
}

func TestConfigRendererCanDisableSelectorAndRouteToCurrentOutbound(t *testing.T) {
	renderer := singbox.NewConfigRenderer()

	configJSON, err := renderer.Render(singbox.RenderInput{
		ListenHost:       "127.0.0.1",
		ListenPort:       18080,
		ControllerPort:   19090,
		ControllerSecret: "secret-1",
		CurrentNodeID:    "1",
		DisableSelector:  true,
		Nodes: []singbox.RuntimeNode{{
			ID:             "1",
			Name:           "HK-A",
			Protocol:       "trojan",
			Server:         "1.1.1.1",
			ServerPort:     443,
			Credential:     []byte(`{"password":"secret"}`),
			TransportJSON:  `{}`,
			TLSJSON:        `{"enabled":true,"server_name":"hk.example.com"}`,
			RawPayloadJSON: `{}`,
		}},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(configJSON, &config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	outbounds := config["outbounds"].([]any)
	for _, item := range outbounds {
		entry := item.(map[string]any)
		if entry["tag"] == "tunnel-selector" {
			t.Fatalf("selector outbound should be omitted: %+v", entry)
		}
	}
	routeConfig := config["route"].(map[string]any)
	if routeConfig["final"] != "node-1" {
		t.Fatalf("route.final = %v, want node-1", routeConfig["final"])
	}
}

func TestConfigCompilerFormatAndCheck(t *testing.T) {
	renderer := singbox.NewConfigRenderer()
	compiler := &singbox.ConfigCompiler{}

	configJSON, err := renderer.Render(singbox.RenderInput{
		ListenHost:       "127.0.0.1",
		ListenPort:       18080,
		ControllerPort:   19090,
		ControllerSecret: "secret-1",
		Nodes: []singbox.RuntimeNode{{
			ID:             "1",
			Name:           "HK-A",
			Protocol:       "trojan",
			Server:         "1.1.1.1",
			ServerPort:     443,
			Credential:     []byte(`{"password":"secret"}`),
			TransportJSON:  `{}`,
			TLSJSON:        `{"enabled":true,"server_name":"hk.example.com"}`,
			RawPayloadJSON: `{}`,
		}},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	formatted, err := compiler.Format(configJSON)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if len(formatted) == 0 {
		t.Fatal("Format() returned empty config")
	}
	if err := compiler.Check(formatted); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if err := compiler.Check([]byte(`{"inbounds":[{"listen_port":"bad"}]}`)); err == nil {
		t.Fatal("Check() invalid config error = nil, want error")
	}
}

func TestPortAllocatorReservesAndReleasesPorts(t *testing.T) {
	allocator := singbox.NewPortAllocator()
	defer func() {
		_ = allocator.Close()
	}()

	pair, err := allocator.AllocatePair()
	if err != nil {
		t.Fatalf("AllocatePair() error = %v", err)
	}
	if pair.ProxyPort == pair.ControllerPort || pair.ProxyPort == 0 || pair.ControllerPort == 0 {
		t.Fatalf("AllocatePair() = %+v, want distinct non-zero ports", pair)
	}

	if _, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", itoa(pair.ProxyPort))); err == nil {
		t.Fatal("reserved proxy port listen error = nil, want address in use")
	}

	if err := allocator.Release(pair.ProxyPort); err != nil {
		t.Fatalf("Release(proxy) error = %v", err)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", itoa(pair.ProxyPort)))
	if err != nil {
		t.Fatalf("listen released proxy port error = %v", err)
	}
	_ = listener.Close()
}

func TestNewRuntimeLayoutIncludesTunnelName(t *testing.T) {
	tempDir := t.TempDir()
	layout := singbox.NewRuntimeGroupLayout(tempDir, "Asia/1", "Proxy A/1")

	wantRoot := filepath.Join(tempDir, "Asia-1-Proxy-A-1")
	if layout.RootDir != wantRoot {
		t.Fatalf("RootDir = %q, want %q", layout.RootDir, wantRoot)
	}
	if layout.ConfigPath != filepath.Join(wantRoot, "config.json") {
		t.Fatalf("ConfigPath = %q, want under runtime root", layout.ConfigPath)
	}
}

func TestSupervisorLifecycleAndLogFiles(t *testing.T) {
	tempDir := t.TempDir()
	layout := singbox.NewRuntimeGroupLayout(tempDir, "asia", "proxy-a")
	compiler := &fakeCompiler{formatted: []byte("{\n  \"log\": {\n    \"level\": \"warn\"\n  }\n}\n")}
	box := &fakeBox{}
	factory := &fakeFactory{box: box}
	supervisor := singbox.NewSupervisor(singbox.SupervisorOptions{
		Compiler: compiler,
		Factory:  factory,
		Now: func() time.Time {
			return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
		},
	})

	if err := supervisor.Start(context.Background(), singbox.StartRequest{
		Layout: layout,
		Config: []byte(`{"raw":true}`),
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if supervisor.Status() != singbox.ProcessStatusRunning {
		t.Fatalf("Status() = %q, want running", supervisor.Status())
	}
	if compiler.formatCalls != 1 || compiler.checkCalls != 1 {
		t.Fatalf("compiler calls = format:%d check:%d, want 1/1", compiler.formatCalls, compiler.checkCalls)
	}
	if box.startCalls != 1 {
		t.Fatalf("box.startCalls = %d, want 1", box.startCalls)
	}

	if _, err := os.Stat(layout.ConfigPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(config) error = %v, want not exist", err)
	}

	factory.writer.WriteMessage(sbLog.LevelTrace, "trace hidden")
	factory.writer.WriteMessage(sbLog.LevelInfo, "[0041] [\x1b[38;5;226m1273129477\x1b[0m 0ms]")
	factory.writer.WriteMessage(sbLog.LevelError, "runtime failed")

	if err := supervisor.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if supervisor.Status() != singbox.ProcessStatusStopped {
		t.Fatalf("Status() after stop = %q, want stopped", supervisor.Status())
	}
	if box.closeCalls != 1 {
		t.Fatalf("box.closeCalls = %d, want 1", box.closeCalls)
	}

	stdout, err := os.ReadFile(layout.StdoutLogPath)
	if err != nil {
		t.Fatalf("ReadFile(stdout) error = %v", err)
	}
	stderr, err := os.ReadFile(layout.StderrLogPath)
	if err != nil {
		t.Fatalf("ReadFile(stderr) error = %v", err)
	}
	if len(stdout) != 0 {
		t.Fatalf("stdout log = %q, want info/trace filtered at warn level", stdout)
	}
	if bytes.Contains(stdout, []byte("\x1b[")) {
		t.Fatalf("stdout log = %q, want ansi escape removed", stdout)
	}
	if !bytes.Contains(stderr, []byte("runtime failed")) {
		t.Fatalf("stderr log = %q, want runtime failed", stderr)
	}
	if bytes.Contains(stderr, []byte("trace hidden")) {
		t.Fatalf("stderr log = %q, want trace filtered", stderr)
	}
}

func TestSupervisorWritesInfoWithoutANSIAtInfoLevel(t *testing.T) {
	tempDir := t.TempDir()
	layout := singbox.NewRuntimeGroupLayout(tempDir, "asia", "proxy-a")
	compiler := &fakeCompiler{formatted: []byte("{\n  \"log\": {\n    \"level\": \"info\"\n  }\n}\n")}
	box := &fakeBox{}
	factory := &fakeFactory{box: box}
	supervisor := singbox.NewSupervisor(singbox.SupervisorOptions{
		Compiler: compiler,
		Factory:  factory,
		Now: func() time.Time {
			return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
		},
	})

	if err := supervisor.Start(context.Background(), singbox.StartRequest{
		Layout: layout,
		Config: []byte(`{"raw":true}`),
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	factory.writer.WriteMessage(sbLog.LevelInfo, "[0041] [\x1b[38;5;226m1273129477\x1b[0m 0ms]")

	if err := supervisor.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stdout, err := os.ReadFile(layout.StdoutLogPath)
	if err != nil {
		t.Fatalf("ReadFile(stdout) error = %v", err)
	}
	if bytes.Contains(stdout, []byte("\x1b[")) {
		t.Fatalf("stdout log = %q, want ansi escape removed", stdout)
	}
	if !bytes.Contains(stdout, []byte("[0041] [1273129477 0ms]")) {
		t.Fatalf("stdout log = %q, want cleaned latency line", stdout)
	}
}

func TestSupervisorStartDetachesRuntimeLifetimeFromCallerContext(t *testing.T) {
	tempDir := t.TempDir()
	layout := singbox.NewRuntimeGroupLayout(tempDir, "asia", "proxy-a")
	compiler := &fakeCompiler{formatted: []byte("{\n  \"ok\": true\n}\n")}
	box := &fakeBox{}
	factory := &fakeFactory{box: box}
	supervisor := singbox.NewSupervisor(singbox.SupervisorOptions{
		Compiler: compiler,
		Factory:  factory,
	})

	type contextKey string
	const requestIDKey contextKey = "request-id"

	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), requestIDKey, "req-1"))
	t.Cleanup(cancelParent)

	if err := supervisor.Start(parent, singbox.StartRequest{
		Layout: layout,
		Config: []byte(`{"raw":true}`),
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		if err := supervisor.Stop(); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	}()

	if factory.ctx == nil {
		t.Fatal("factory ctx is nil")
	}
	if got := factory.ctx.Value(requestIDKey); got != "req-1" {
		t.Fatalf("factory ctx value = %v, want req-1", got)
	}

	cancelParent()

	select {
	case <-factory.ctx.Done():
		t.Fatal("runtime context should not be canceled when caller context ends")
	default:
	}
}

func TestClashAPIClientGetAndSwitchSelector(t *testing.T) {
	var gotAuth string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/proxies/tunnel-selector":
			_, _ = w.Write([]byte(`{"type":"Selector","name":"tunnel-selector","now":"node-1","all":["node-1","node-2"]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/proxies/tunnel-selector":
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := singbox.NewClashAPIClient(server.URL, "secret-1", server.Client())
	info, err := client.GetProxy(context.Background(), "tunnel-selector")
	if err != nil {
		t.Fatalf("GetProxy() error = %v", err)
	}
	if info.Now != "node-1" || len(info.All) != 2 {
		t.Fatalf("GetProxy() = %+v, want selector payload", info)
	}
	if err := client.SwitchSelector(context.Background(), "tunnel-selector", "node-2"); err != nil {
		t.Fatalf("SwitchSelector() error = %v", err)
	}
	if gotAuth != "Bearer secret-1" {
		t.Fatalf("Authorization = %q, want Bearer secret-1", gotAuth)
	}
	if !bytes.Contains(gotBody, []byte(`"name":"node-2"`)) {
		t.Fatalf("switch body = %q, want node-2", gotBody)
	}
}

func TestMapRuntimeError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{err: nil, want: domain.TunnelStatusRunning},
		{err: context.Canceled, want: domain.TunnelStatusStopped},
		{err: fmt.Errorf("%w: bad request", singbox.ErrSelectorSwitch), want: domain.TunnelStatusDegraded},
		{err: errors.New("boom"), want: domain.TunnelStatusError},
	}

	for _, tc := range cases {
		if got := singbox.MapRuntimeError(tc.err); got != tc.want {
			t.Fatalf("MapRuntimeError(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

type fakeCompiler struct {
	formatted   []byte
	formatCalls int
	checkCalls  int
}

func (f *fakeCompiler) Format(config []byte) ([]byte, error) {
	f.formatCalls++
	return f.formatted, nil
}

func (f *fakeCompiler) Check(config []byte) error {
	f.checkCalls++
	if !bytes.Equal(config, f.formatted) {
		return errors.New("unexpected config for check")
	}
	return nil
}

type fakeBox struct {
	startCalls int
	closeCalls int
}

func (f *fakeBox) Start() error {
	f.startCalls++
	return nil
}

func (f *fakeBox) Close() error {
	f.closeCalls++
	return nil
}

type fakeFactory struct {
	box    *fakeBox
	ctx    context.Context
	writer sbLog.PlatformWriter
}

func (f *fakeFactory) New(ctx context.Context, config []byte, writer sbLog.PlatformWriter) (singbox.BoxInstance, error) {
	f.ctx = ctx
	f.writer = writer
	return f.box, nil
}

func itoa(port int) string {
	return fmt.Sprintf("%d", port)
}
