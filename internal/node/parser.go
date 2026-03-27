package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func ParseImportPayload(payload string) ([]ImportedNode, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, ErrInvalidPayload
	}
	lines := splitImportLines(payload)
	result := make([]ImportedNode, 0, len(lines))
	for _, line := range lines {
		item, err := parseImportLine(line)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func splitImportLines(payload string) []string {
	var lines []string
	for _, line := range strings.Split(tryDecodeWholePayload(payload), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func tryDecodeWholePayload(payload string) string {
	if strings.Contains(payload, "://") {
		return payload
	}
	if decoded, err := decodeBase64String(payload); err == nil && strings.Contains(decoded, "://") {
		return decoded
	}
	return payload
}

func parseImportLine(line string) (ImportedNode, error) {
	switch {
	case strings.HasPrefix(line, "ss://"):
		return parseSS(line)
	case strings.HasPrefix(line, "trojan://"):
		return parseTrojan(line)
	case strings.HasPrefix(line, "vmess://"):
		return parseVMess(line)
	case strings.HasPrefix(line, "vless://"):
		return parseVLESS(line)
	case strings.HasPrefix(line, "hysteria2://"), strings.HasPrefix(line, "hy2://"):
		return parseHysteria2(line)
	default:
		return ImportedNode{}, ErrUnsupportedProtocol
	}
}

func parseSS(line string) (ImportedNode, error) {
	raw := strings.TrimPrefix(line, "ss://")
	name := ""
	if index := strings.Index(raw, "#"); index >= 0 {
		name, _ = url.QueryUnescape(raw[index+1:])
		raw = raw[:index]
	}
	query := ""
	if index := strings.Index(raw, "?"); index >= 0 {
		query = raw[index+1:]
		raw = raw[:index]
	}
	decoded := raw
	if !strings.Contains(decoded, "@") {
		var err error
		decoded, err = decodeBase64String(decoded)
		if err != nil {
			return ImportedNode{}, ErrInvalidPayload
		}
	}
	parts := strings.SplitN(decoded, "@", 2)
	if len(parts) != 2 {
		return ImportedNode{}, ErrInvalidPayload
	}
	cred := strings.SplitN(parts[0], ":", 2)
	if len(cred) != 2 {
		return ImportedNode{}, ErrInvalidPayload
	}
	host, port, err := splitHostPort(parts[1])
	if err != nil {
		return ImportedNode{}, err
	}
	transportJSON, _ := normalizeJSON(`{}`)
	tlsJSON, _ := normalizeJSON(`{}`)
	rawJSON, _ := normalizeJSON(fmt.Sprintf(`{"plugin":"%s"}`, queryValue(query, "plugin")))
	credential, _ := json.Marshal(map[string]any{
		"method":   cred[0],
		"password": cred[1],
	})
	dedupe := ComputeDedupeFingerprint("ss", host, port, credential, transportJSON, tlsJSON)
	return ImportedNode{
		Name:              fallbackName(name, "ss-"+host),
		Protocol:          "ss",
		Server:            host,
		ServerPort:        port,
		Credential:        credential,
		TransportJSON:     transportJSON,
		TLSJSON:           tlsJSON,
		RawPayloadJSON:    rawJSON,
		DedupeFingerprint: dedupe,
	}, nil
}

func parseTrojan(line string) (ImportedNode, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return ImportedNode{}, ErrInvalidPayload
	}
	query := parsed.Query()
	port, err := parsePort(parsed.Port())
	if err != nil {
		return ImportedNode{}, err
	}
	transportJSON, _ := normalizeJSONMap(buildTransportOptions(
		fallbackName(query.Get("type"), "tcp"),
		query.Get("host"),
		query.Get("path"),
		firstNonEmpty(query.Get("serviceName"), query.Get("service_name")),
	))
	tlsJSON, _ := normalizeJSONMap(buildTLSOptions(
		true,
		fallbackName(query.Get("sni"), query.Get("peer")),
		queryBool(query, "allowInsecure", "allow_insecure", "skip-cert-verify", "insecure"),
	))
	rawJSON, _ := normalizeJSON(fmt.Sprintf(`{"security":"%s"}`, query.Get("security")))
	credential, _ := json.Marshal(map[string]any{
		"password": parsed.User.Username(),
	})
	dedupe := ComputeDedupeFingerprint("trojan", parsed.Hostname(), port, credential, transportJSON, tlsJSON)
	return ImportedNode{
		Name:              fallbackName(fragmentName(parsed), "trojan-"+parsed.Hostname()),
		Protocol:          "trojan",
		Server:            parsed.Hostname(),
		ServerPort:        port,
		Credential:        credential,
		TransportJSON:     transportJSON,
		TLSJSON:           tlsJSON,
		RawPayloadJSON:    rawJSON,
		DedupeFingerprint: dedupe,
	}, nil
}

func parseVMess(line string) (ImportedNode, error) {
	raw := strings.TrimPrefix(line, "vmess://")
	decoded, err := decodeBase64String(raw)
	if err != nil {
		return ImportedNode{}, ErrInvalidPayload
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
		return ImportedNode{}, ErrInvalidPayload
	}
	port, err := parsePort(payload["port"])
	if err != nil {
		return ImportedNode{}, err
	}
	transportJSON, _ := normalizeJSONMap(buildTransportOptions(
		fallbackName(payload["net"], "tcp"),
		payload["host"],
		payload["path"],
		firstNonEmpty(payload["serviceName"], payload["service_name"]),
	))
	tlsEnabled := payload["tls"] != ""
	tlsJSON, _ := normalizeJSONMap(buildTLSOptions(tlsEnabled, payload["sni"], parseBoolString(payload["allowInsecure"])))
	rawJSON, _ := normalizeJSON(fmt.Sprintf(`{"aid":"%s","scy":"%s"}`, payload["aid"], payload["scy"]))
	credential, _ := json.Marshal(map[string]any{
		"uuid": payload["id"],
	})
	dedupe := ComputeDedupeFingerprint("vmess", payload["add"], port, credential, transportJSON, tlsJSON)
	return ImportedNode{
		Name:              fallbackName(payload["ps"], "vmess-"+payload["add"]),
		Protocol:          "vmess",
		Server:            payload["add"],
		ServerPort:        port,
		Credential:        credential,
		TransportJSON:     transportJSON,
		TLSJSON:           tlsJSON,
		RawPayloadJSON:    rawJSON,
		DedupeFingerprint: dedupe,
	}, nil
}

func parseVLESS(line string) (ImportedNode, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return ImportedNode{}, ErrInvalidPayload
	}
	query := parsed.Query()
	port, err := parsePort(parsed.Port())
	if err != nil {
		return ImportedNode{}, err
	}
	transportJSON, _ := normalizeJSONMap(buildTransportOptions(
		fallbackName(query.Get("type"), "tcp"),
		query.Get("host"),
		query.Get("path"),
		firstNonEmpty(query.Get("serviceName"), query.Get("service_name")),
	))
	tlsJSON, _ := normalizeJSONMap(buildTLSOptions(
		query.Get("security") == "tls",
		fallbackName(query.Get("sni"), query.Get("host")),
		queryBool(query, "allowInsecure", "allow_insecure", "skip-cert-verify", "insecure"),
	))
	rawJSON, _ := normalizeJSON(fmt.Sprintf(`{"flow":"%s","encryption":"%s"}`, query.Get("flow"), query.Get("encryption")))
	credential, _ := json.Marshal(map[string]any{
		"uuid": parsed.User.Username(),
	})
	dedupe := ComputeDedupeFingerprint("vless", parsed.Hostname(), port, credential, transportJSON, tlsJSON)
	return ImportedNode{
		Name:              fallbackName(fragmentName(parsed), "vless-"+parsed.Hostname()),
		Protocol:          "vless",
		Server:            parsed.Hostname(),
		ServerPort:        port,
		Credential:        credential,
		TransportJSON:     transportJSON,
		TLSJSON:           tlsJSON,
		RawPayloadJSON:    rawJSON,
		DedupeFingerprint: dedupe,
	}, nil
}

func parseHysteria2(line string) (ImportedNode, error) {
	line = strings.Replace(line, "hy2://", "hysteria2://", 1)
	parsed, err := url.Parse(line)
	if err != nil {
		return ImportedNode{}, ErrInvalidPayload
	}
	query := parsed.Query()
	port, err := parsePort(parsed.Port())
	if err != nil {
		return ImportedNode{}, err
	}
	transportJSON, _ := normalizeJSON(fmt.Sprintf(`{"obfs":"%s","obfs_password":"%s"}`, query.Get("obfs"), query.Get("obfs-password")))
	tlsJSON, _ := normalizeJSONMap(buildTLSOptions(
		true,
		fallbackName(query.Get("sni"), parsed.Hostname()),
		queryBool(query, "insecure", "allowInsecure", "allow_insecure", "skip-cert-verify"),
	))
	rawJSON, _ := normalizeJSON(fmt.Sprintf(`{"up_mbps":"%s","down_mbps":"%s"}`, query.Get("upmbps"), query.Get("downmbps")))
	credential, _ := json.Marshal(map[string]any{
		"password": parsed.User.Username(),
	})
	dedupe := ComputeDedupeFingerprint("hysteria2", parsed.Hostname(), port, credential, transportJSON, tlsJSON)
	return ImportedNode{
		Name:              fallbackName(fragmentName(parsed), "hy2-"+parsed.Hostname()),
		Protocol:          "hysteria2",
		Server:            parsed.Hostname(),
		ServerPort:        port,
		Credential:        credential,
		TransportJSON:     transportJSON,
		TLSJSON:           tlsJSON,
		RawPayloadJSON:    rawJSON,
		DedupeFingerprint: dedupe,
	}, nil
}

func decodeBase64String(value string) (string, error) {
	value = strings.TrimSpace(value)
	for _, codec := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if decoded, err := codec.DecodeString(value); err == nil {
			return string(decoded), nil
		}
	}
	return "", ErrInvalidPayload
}

func splitHostPort(value string) (string, int, error) {
	host, portString, err := net.SplitHostPort(value)
	if err != nil {
		if !strings.Contains(value, ":") {
			return "", 0, ErrInvalidPayload
		}
		last := strings.LastIndex(value, ":")
		host = value[:last]
		portString = value[last+1:]
	}
	port, err := parsePort(portString)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port <= 0 {
		return 0, ErrInvalidPayload
	}
	return port, nil
}

func fragmentName(value *url.URL) string {
	if value == nil {
		return ""
	}
	name, _ := url.QueryUnescape(value.Fragment)
	return name
}

func fallbackName(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func queryValue(raw, key string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	return values.Get(key)
}

func normalizeJSONMap(payload map[string]any) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return normalizeJSON(string(raw))
}

func buildTransportOptions(transportType, host, path, serviceName string) map[string]any {
	result := map[string]any{
		"type": transportType,
	}
	if strings.TrimSpace(host) != "" {
		result["host"] = host
	}
	if strings.TrimSpace(path) != "" {
		result["path"] = path
	}
	if strings.TrimSpace(serviceName) != "" {
		result["service_name"] = serviceName
	}
	return result
}

func buildTLSOptions(enabled bool, serverName string, insecure bool) map[string]any {
	result := map[string]any{
		"enabled":     enabled,
		"server_name": serverName,
	}
	if insecure {
		result["insecure"] = true
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func queryBool(values url.Values, keys ...string) bool {
	for _, key := range keys {
		if parseBoolString(values.Get(key)) {
			return true
		}
	}
	return false
}

func parseBoolString(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
