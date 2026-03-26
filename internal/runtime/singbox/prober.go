package singbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	sbbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	sbjson "github.com/sagernet/sing/common/json"

	"github.com/WAY29/SimplePool/internal/node"
)

type Prober struct {
	testURL string
	timeout time.Duration
}

func NewProber(testURL string, timeout time.Duration) *Prober {
	if testURL == "" {
		testURL = "https://cloudflare.com/cdn-cgi/trace"
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Prober{
		testURL: testURL,
		timeout: timeout,
	}
}

func (p *Prober) Probe(ctx context.Context, target node.ProbeTarget) (node.ProbeResult, error) {
	inboundPort, err := allocatePort()
	if err != nil {
		return node.ProbeResult{TestURL: p.testURL}, err
	}

	configJSON, err := p.buildConfig(target, inboundPort)
	if err != nil {
		return node.ProbeResult{TestURL: p.testURL}, err
	}

	singCtx, cancel := context.WithCancel(include.Context(context.Background()))
	defer cancel()

	options, err := sbjson.UnmarshalExtendedContext[option.Options](singCtx, configJSON)
	if err != nil {
		return node.ProbeResult{TestURL: p.testURL}, err
	}

	instance, err := sbbox.New(sbbox.Options{
		Context: singCtx,
		Options: options,
	})
	if err != nil {
		return node.ProbeResult{TestURL: p.testURL}, err
	}
	defer instance.Close()

	if err := instance.Start(); err != nil {
		return node.ProbeResult{TestURL: p.testURL}, err
	}

	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", inboundPort))
	client := &http.Client{
		Timeout: p.timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.testURL, nil)
	if err != nil {
		return node.ProbeResult{TestURL: p.testURL}, err
	}

	startedAt := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return node.ProbeResult{
			TestURL:      p.testURL,
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)

	result := node.ProbeResult{
		TestURL:   p.testURL,
		Success:   response.StatusCode >= 200 && response.StatusCode < 400,
		LatencyMS: time.Since(startedAt).Milliseconds(),
	}
	if !result.Success {
		result.ErrorMessage = response.Status
	}
	return result, nil
}

func (p *Prober) buildConfig(target node.ProbeTarget, inboundPort int) ([]byte, error) {
	outbound, err := buildOutbound(target)
	if err != nil {
		return nil, err
	}

	config := map[string]any{
		"inbounds": []map[string]any{
			{
				"type":        "http",
				"tag":         "probe-in",
				"listen":      "127.0.0.1",
				"listen_port": inboundPort,
			},
		},
		"outbounds": []any{
			outbound,
		},
		"route": map[string]any{
			"final": "probe-out",
		},
	}
	return json.Marshal(config)
}

func buildOutbound(target node.ProbeTarget) (map[string]any, error) {
	return buildRuntimeOutbound("probe-out", RuntimeNode{
		ID:             target.ID,
		Name:           target.Name,
		Protocol:       target.Protocol,
		Server:         target.Server,
		ServerPort:     target.ServerPort,
		Credential:     target.Credential,
		TransportJSON:  target.TransportJSON,
		TLSJSON:        target.TLSJSON,
		RawPayloadJSON: target.RawPayloadJSON,
	})
}

func buildTransportMap(transport map[string]any) map[string]any {
	transportType := stringValue(transport["type"], "")
	switch transportType {
	case "", "tcp":
		return nil
	case "ws":
		result := map[string]any{"type": "ws"}
		if path := stringValue(transport["path"], ""); path != "" {
			result["path"] = path
		}
		if host := stringValue(transport["host"], ""); host != "" {
			result["headers"] = map[string]any{"Host": host}
		}
		return result
	case "http":
		result := map[string]any{"type": "http"}
		if path := stringValue(transport["path"], ""); path != "" {
			result["path"] = path
		}
		if host := stringValue(transport["host"], ""); host != "" {
			result["host"] = []string{host}
		}
		return result
	case "grpc":
		result := map[string]any{"type": "grpc"}
		if serviceName := stringValue(transport["service_name"], ""); serviceName != "" {
			result["service_name"] = serviceName
		}
		return result
	default:
		return nil
	}
}

func buildTLSMap(options map[string]any) map[string]any {
	if !boolValue(options["enabled"]) {
		return nil
	}
	result := map[string]any{
		"enabled": true,
	}
	if serverName := stringValue(options["server_name"], ""); serverName != "" {
		result["server_name"] = serverName
	}
	if boolValue(options["insecure"]) {
		result["insecure"] = true
	}
	return result
}

func decodeJSONMap(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func normalizeProtocol(protocol string) string {
	if protocol == "hy2" {
		return "hysteria2"
	}
	return protocol
}

func stringValue(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return fallback
		}
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	default:
		return false
	}
}

func intValue(value any) (int, error) {
	switch typed := value.(type) {
	case float64:
		return int(typed), nil
	case int:
		return typed, nil
	case string:
		if typed == "" {
			return 0, fmt.Errorf("empty")
		}
		return strconv.Atoi(typed)
	default:
		return 0, fmt.Errorf("unsupported")
	}
}

func allocatePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
