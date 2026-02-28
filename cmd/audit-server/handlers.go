package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type Server struct {
	cfg        Config
	db         *gorm.DB
	listTmpl   *template.Template
	detailTmpl *template.Template
}

func (s *Server) routes(staticHandler http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /webhook/newapi", s.handleNewAPIWebhook)

	// static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler))

	// UI
	mux.Handle("GET /", s.withAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/events", http.StatusFound)
	})))
	mux.Handle("GET /events", s.withAuth(http.HandlerFunc(s.handleEventsListPage)))
	mux.Handle("GET /events/{id}", s.withAuth(http.HandlerFunc(s.handleEventDetailPage)))

	// API
	mux.Handle("GET /api/events", s.withAuth(http.HandlerFunc(s.handleEventsListAPI)))
	mux.Handle("GET /api/events/{id}", s.withAuth(http.HandlerFunc(s.handleEventDetailAPI)))

	return mux
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	// If not configured, allow access (but still usable behind reverse proxy).
	if strings.TrimSpace(s.cfg.AuthToken) == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(s.cfg.AuthToken)
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			if strings.TrimSpace(auth[len("bearer "):]) == token {
				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

func (s *Server) handleNewAPIWebhook(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if r.Body == nil {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, s.cfg.MaxBodyBytes+1))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > s.cfg.MaxBodyBytes {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	tsHeader := strings.TrimSpace(r.Header.Get("X-NewAPI-Audit-Timestamp"))
	sigHeader := strings.TrimSpace(r.Header.Get("X-NewAPI-Audit-Signature"))

	if secret := strings.TrimSpace(s.cfg.WebhookSecret); secret != "" {
		if tsHeader == "" || sigHeader == "" {
			http.Error(w, "missing signature", http.StatusUnauthorized)
			return
		}

		ts, err := strconv.ParseInt(tsHeader, 10, 64)
		if err != nil {
			http.Error(w, "invalid timestamp", http.StatusUnauthorized)
			return
		}

		now := time.Now().Unix()
		if s.cfg.MaxSkewSeconds > 0 {
			if absInt64(now-ts) > s.cfg.MaxSkewSeconds {
				http.Error(w, "timestamp skew too large", http.StatusUnauthorized)
				return
			}
		}

		if !verifyNewAPISignature(secret, tsHeader, body, sigHeader) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var event NewAPIAuditEvent
	if err := common.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	record := AuditRecord{
		RemoteIP:       s.getRemoteIP(r),
		UserAgent:      truncateString(strings.TrimSpace(r.UserAgent()), 512),
		AuditTimestamp: parseInt64(tsHeader),
		Signature:      truncateString(sigHeader, 128),

		EventType:      truncateString(event.Type, 64),
		EventTimestamp: event.Timestamp,

		RequestID:  truncateString(event.RequestId, 64),
		Method:     truncateString(event.Method, 16),
		Path:       truncateString(event.Path, 512),
		StatusCode: event.StatusCode,
		DurationMs: event.DurationMs,

		UserID:     event.UserId,
		Username:   truncateString(event.Username, 128),
		TokenID:    event.TokenId,
		TokenName:  truncateString(event.TokenName, 256),
		UsingGroup: truncateString(event.Group, 128),

		ChannelID:   event.ChannelId,
		ChannelName: truncateString(event.ChannelName, 256),
		ChannelType: event.ChannelType,
		Model:       truncateString(event.Model, 256),

		ContentType:          truncateString(event.ContentType, 256),
		RequestBody:          event.RequestBody,
		RequestBodyEncoding:  truncateString(event.RequestBodyEncoding, 16),
		RequestBodyBytes:     event.RequestBodyBytes,
		RequestBodyTruncated: event.RequestBodyTruncated,

		RawPayload: string(body),
	}

	if err := s.db.Create(&record).Error; err != nil {
		log.Printf("insert audit record failed: %v", err)
		http.Error(w, "store failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEventsListAPI(w http.ResponseWriter, r *http.Request) {
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	beforeID := parseUint64(r.URL.Query().Get("before_id"))

	var records []AuditRecord
	q := s.db.Model(&AuditRecord{}).Order("id desc")
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	if requestID := strings.TrimSpace(r.URL.Query().Get("request_id")); requestID != "" {
		q = q.Where("request_id = ?", requestID)
	}
	if path := strings.TrimSpace(r.URL.Query().Get("path")); path != "" {
		q = q.Where("path = ?", path)
	}
	if userIDStr := strings.TrimSpace(r.URL.Query().Get("user_id")); userIDStr != "" {
		if userID := parseInt(userIDStr, 0); userID > 0 {
			q = q.Where("user_id = ?", userID)
		}
	}
	if statusStr := strings.TrimSpace(r.URL.Query().Get("status_code")); statusStr != "" {
		if status := parseInt(statusStr, 0); status > 0 {
			q = q.Where("status_code = ?", status)
		}
	}

	if err := q.Limit(limit).Find(&records).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": "query failed",
		})
		return
	}

	var nextBeforeID uint64
	if len(records) > 0 {
		nextBeforeID = records[len(records)-1].ID
	}

	// Avoid sending raw payload by default (can be large).
	summaries := make([]AuditRecordSummary, 0, len(records))
	for _, rec := range records {
		summaries = append(summaries, rec.Summary())
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"data":           summaries,
		"next_before_id": nextBeforeID,
	})
}

func (s *Server) handleEventDetailAPI(w http.ResponseWriter, r *http.Request) {
	id := parseUint64(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "invalid id"})
		return
	}
	var record AuditRecord
	err := s.db.First(&record, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": "query failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": record})
}

func (s *Server) handleEventsListPage(w http.ResponseWriter, r *http.Request) {
	limit := parseInt(r.URL.Query().Get("limit"), 100)
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	beforeID := parseUint64(r.URL.Query().Get("before_id"))
	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))

	var records []AuditRecord
	q := s.db.Model(&AuditRecord{}).Order("id desc")
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	if requestID != "" {
		q = q.Where("request_id = ?", requestID)
	}

	if err := q.Limit(limit).Find(&records).Error; err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	var nextBeforeID uint64
	if len(records) > 0 {
		nextBeforeID = records[len(records)-1].ID
	}

	data := ListPageData{
		Events:       records,
		Limit:        limit,
		RequestID:    requestID,
		BeforeID:     beforeID,
		NextBeforeID: nextBeforeID,
	}

	if err := s.listTmpl.ExecuteTemplate(w, "list.html", data); err != nil {
		log.Printf("render list page failed: %v", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleEventDetailPage(w http.ResponseWriter, r *http.Request) {
	id := parseUint64(r.PathValue("id"))
	if id == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var record AuditRecord
	err := s.db.First(&record, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	data := DetailPageData{
		Event: record,
	}
	if err := s.detailTmpl.ExecuteTemplate(w, "detail.html", data); err != nil {
		log.Printf("render detail page failed: %v", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
}

type ListPageData struct {
	Events []AuditRecord

	Limit     int
	RequestID string

	BeforeID     uint64
	NextBeforeID uint64
}

type DetailPageData struct {
	Event AuditRecord
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	payload, err := common.Marshal(v)
	if err != nil {
		http.Error(w, "marshal failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func verifyNewAPISignature(secret, timestamp string, payload []byte, sigHeader string) bool {
	if secret == "" {
		return true
	}
	sig := strings.TrimSpace(sigHeader)
	sig = strings.TrimPrefix(sig, "sha256=")
	recv, err := hex.DecodeString(sig)
	if err != nil || len(recv) == 0 {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, recv)
}

func (s *Server) getRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if s.cfg.TrustProxyHeaders {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			// XFF may be a list, client first.
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				if ip := strings.TrimSpace(parts[0]); ip != "" {
					return truncateString(ip, 128)
				}
			}
		}
		if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
			return truncateString(xrip, 128)
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return truncateString(host, 128)
	}
	return truncateString(strings.TrimSpace(r.RemoteAddr), 128)
}

func parseInt64(v string) int64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	i, _ := strconv.ParseInt(v, 10, 64)
	return i
}

func parseInt(v string, defaultValue int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return i
}

func parseUint64(v string) uint64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	i, _ := strconv.ParseUint(v, 10, 64)
	return i
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}
