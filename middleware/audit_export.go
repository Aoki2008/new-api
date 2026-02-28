package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const auditBodyMaxHardLimitBytes int64 = 1024 * 1024

type auditTapReadCloser struct {
	rc        io.ReadCloser
	buf       *bytes.Buffer
	limit     int64
	truncated bool
}

func (t *auditTapReadCloser) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n <= 0 || t.buf == nil || t.limit <= 0 || t.truncated {
		return n, err
	}

	remaining := t.limit - int64(t.buf.Len())
	if remaining <= 0 {
		t.truncated = true
		return n, err
	}

	toCopy := n
	if int64(toCopy) > remaining {
		toCopy = int(remaining)
		t.truncated = true
	}
	_, _ = t.buf.Write(p[:toCopy])
	if int64(n) > remaining {
		t.truncated = true
	}
	return n, err
}

func (t *auditTapReadCloser) Close() error {
	return t.rc.Close()
}

func AuditExport() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil {
			return
		}
		if !common.LogRequestBodyEnabled {
			c.Next()
			return
		}
		if strings.TrimSpace(common.AuditWebhookUrl) == "" {
			c.Next()
			return
		}
		if c.Request == nil || c.Request.Body == nil || c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		contentType := c.Request.Header.Get("Content-Type")
		// Avoid capturing multipart bodies (may contain binary file data).
		skipBodyCapture := strings.Contains(contentType, gin.MIMEMultipartPOSTForm)

		maxBytes := int64(common.LogRequestBodyMaxBytes)
		if maxBytes < 0 {
			maxBytes = 0
		}
		if maxBytes > auditBodyMaxHardLimitBytes {
			maxBytes = auditBodyMaxHardLimitBytes
		}
		if skipBodyCapture {
			maxBytes = 0
		}

		var buf *bytes.Buffer
		if maxBytes > 0 {
			buf = &bytes.Buffer{}
		}

		tap := &auditTapReadCloser{
			rc:    c.Request.Body,
			buf:   buf,
			limit: maxBytes,
		}
		c.Request.Body = tap

		start := time.Now()
		c.Next()

		var bodyBytes []byte
		if tap.buf != nil {
			bodyBytes = tap.buf.Bytes()
		}

		bodyEncoding := "utf8"
		bodyPreview := ""
		if len(bodyBytes) > 0 {
			if utf8.Valid(bodyBytes) {
				bodyPreview = string(bodyBytes)
			} else {
				bodyPreview = base64.StdEncoding.EncodeToString(bodyBytes)
				bodyEncoding = "base64"
			}
		}

		requestID := c.GetString(common.RequestIdKey)
		event := service.AuditEvent{
			Type:      "request_audit",
			Timestamp: time.Now().Unix(),
			RequestId: requestID,
			Method:    c.Request.Method,
			Path: func() string {
				if c.Request.URL != nil {
					return c.Request.URL.Path
				}
				return ""
			}(),
			StatusCode: c.Writer.Status(),
			DurationMs: time.Since(start).Milliseconds(),

			UserId:    c.GetInt("id"),
			Username:  c.GetString("username"),
			TokenId:   c.GetInt("token_id"),
			TokenName: c.GetString("token_name"),
			Group:     c.GetString("group"),

			ChannelId:   c.GetInt("channel_id"),
			ChannelName: c.GetString("channel_name"),
			ChannelType: c.GetInt("channel_type"),
			Model:       c.GetString("original_model"),

			ContentType:          contentType,
			RequestBody:          bodyPreview,
			RequestBodyEncoding:  bodyEncoding,
			RequestBodyBytes:     len(bodyBytes),
			RequestBodyTruncated: tap.truncated,
		}

		// Export asynchronously to avoid adding latency to the main request path.
		exportCtx := context.WithValue(context.Background(), common.RequestIdKey, requestID)
		gopool.Go(func() {
			if err := service.ExportAuditEvent(exportCtx, event); err != nil {
				logger.LogError(exportCtx, "audit export failed: "+err.Error())
			}
		})
	}
}
