package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

type AuditEvent struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`

	RequestId  string `json:"request_id,omitempty"`
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`

	UserId    int    `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenId   int    `json:"token_id,omitempty"`
	TokenName string `json:"token_name,omitempty"`
	Group     string `json:"group,omitempty"`

	ChannelId   int    `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	ChannelType int    `json:"channel_type,omitempty"`
	Model       string `json:"model,omitempty"`

	ContentType          string `json:"content_type,omitempty"`
	RequestBody          string `json:"request_body,omitempty"`
	RequestBodyEncoding  string `json:"request_body_encoding,omitempty"`
	RequestBodyBytes     int    `json:"request_body_bytes,omitempty"`
	RequestBodyTruncated bool   `json:"request_body_truncated,omitempty"`
}

func signAuditPayload(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func ExportAuditEvent(ctx context.Context, event AuditEvent) error {
	webhookURL := strings.TrimSpace(common.AuditWebhookUrl)
	if webhookURL == "" {
		return fmt.Errorf("audit webhook url not configured")
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(webhookURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("audit webhook url rejected: %v", err)
	}

	payload, err := common.Marshal(event)
	if err != nil {
		return err
	}

	timeout := time.Duration(common.AuditWebhookTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("X-NewAPI-Audit-Timestamp", ts)
	if event.RequestId != "" {
		req.Header.Set("X-NewAPI-Request-Id", event.RequestId)
	}
	if secret := strings.TrimSpace(common.AuditWebhookSecret); secret != "" {
		req.Header.Set("X-NewAPI-Audit-Signature", "sha256="+signAuditPayload(secret, ts, payload))
	}

	client := GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("audit webhook response status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
