package messagequeue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultHTTPTimeout = 10 * time.Second
)

func tcpProbe(ctx context.Context, address string, timeout time.Duration) error {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func firstEndpoint(instance *MQInstance, defaultPort int) string {
	endpoints := splitEndpoints(instance.Endpoint)
	if len(endpoints) == 0 {
		return ""
	}
	return normalizeAddress(endpoints[0], defaultPort)
}

func normalizeAddress(raw string, defaultPort int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		raw = parsed.Host
	}
	if _, _, err := net.SplitHostPort(raw); err == nil {
		return raw
	}
	if defaultPort > 0 {
		return net.JoinHostPort(raw, strconv.Itoa(defaultPort))
	}
	return raw
}

func managementBaseURL(instance *MQInstance, defaultPort int, tlsEnabled bool) string {
	if strings.TrimSpace(instance.ManagementURL) != "" {
		return strings.TrimRight(strings.TrimSpace(instance.ManagementURL), "/")
	}
	endpoint := firstEndpoint(instance, defaultPort)
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = strings.TrimSpace(endpoint)
	}
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, strconv.Itoa(defaultPort)))
}

func httpClient(tlsEnabled bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsEnabled {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Timeout: defaultHTTPTimeout, Transport: transport}
}

func doJSONRequest(ctx context.Context, client *http.Client, method, target string, credential *ConnectionCredential, out any) error {
	return doJSONRequestWithBody(ctx, client, method, target, credential, nil, out)
}

func doJSONRequestWithBody(ctx context.Context, client *http.Client, method, target string, credential *ConnectionCredential, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	applyHTTPAuth(req, credential)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("管理接口返回 %d: %s", resp.StatusCode, trimForError(string(respBody)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("解析管理接口响应失败: %w", err)
	}
	return nil
}

func applyHTTPAuth(req *http.Request, credential *ConnectionCredential) {
	if credential == nil {
		return
	}
	if credential.Username != "" || credential.Password != "" {
		req.SetBasicAuth(credential.Username, credential.Password)
		return
	}
	if credential.PrivateKey != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(credential.PrivateKey))
		return
	}
}

func mustJSON(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func trimForError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 300 {
		return value[:300]
	}
	return value
}

func stringValue(v any) string {
	switch item := v.(type) {
	case string:
		return item
	case float64:
		return strconv.FormatFloat(item, 'f', -1, 64)
	case int:
		return strconv.Itoa(item)
	case int64:
		return strconv.FormatInt(item, 10)
	default:
		if item == nil {
			return ""
		}
		return fmt.Sprint(item)
	}
}

func intValue(v any) int {
	switch item := v.(type) {
	case float64:
		return int(item)
	case int:
		return item
	case int64:
		return int(item)
	case json.Number:
		n, _ := item.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(item)
		return n
	default:
		return 0
	}
}

func int64Value(v any) int64 {
	switch item := v.(type) {
	case float64:
		return int64(item)
	case int:
		return int64(item)
	case int64:
		return item
	case json.Number:
		n, _ := item.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(item, 10, 64)
		return n
	default:
		return 0
	}
}

func floatValue(v any) float64 {
	switch item := v.(type) {
	case float64:
		return item
	case int:
		return float64(item)
	case int64:
		return float64(item)
	case json.Number:
		n, _ := item.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(item, 64)
		return n
	default:
		return 0
	}
}

func truncatePayload(data []byte, maxBytes int) (string, bool, string) {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
	}
	if utf8.Valid(data) {
		return string(data), truncated, "text"
	}
	return base64.StdEncoding.EncodeToString(data), truncated, "base64"
}
