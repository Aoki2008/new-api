package main

import "time"

type NewAPIAuditEvent struct {
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

type AuditRecord struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`

	RemoteIP  string `gorm:"size:128" json:"remote_ip"`
	UserAgent string `gorm:"size:512" json:"user_agent"`

	AuditTimestamp int64  `gorm:"index" json:"audit_timestamp"`
	Signature      string `gorm:"size:128" json:"signature"`

	EventType      string `gorm:"size:64;index;column:event_type" json:"type"`
	EventTimestamp int64  `gorm:"index;column:event_timestamp" json:"timestamp"`

	RequestID  string `gorm:"size:64;index;column:request_id" json:"request_id"`
	Method     string `gorm:"size:16;index" json:"method"`
	Path       string `gorm:"size:512;index" json:"path"`
	StatusCode int    `gorm:"index" json:"status_code"`
	DurationMs int64  `json:"duration_ms"`

	UserID     int    `gorm:"index;column:user_id" json:"user_id"`
	Username   string `gorm:"size:128;index" json:"username"`
	TokenID    int    `gorm:"index;column:token_id" json:"token_id"`
	TokenName  string `gorm:"size:256" json:"token_name"`
	UsingGroup string `gorm:"size:128;index;column:using_group" json:"group"`

	ChannelID   int    `gorm:"index;column:channel_id" json:"channel_id"`
	ChannelName string `gorm:"size:256;index" json:"channel_name"`
	ChannelType int    `gorm:"index;column:channel_type" json:"channel_type"`
	Model       string `gorm:"size:256;index" json:"model"`

	ContentType          string `gorm:"size:256" json:"content_type"`
	RequestBody          string `gorm:"type:text" json:"request_body"`
	RequestBodyEncoding  string `gorm:"size:16" json:"request_body_encoding"`
	RequestBodyBytes     int    `json:"request_body_bytes"`
	RequestBodyTruncated bool   `json:"request_body_truncated"`

	RawPayload string `gorm:"type:text" json:"raw_payload,omitempty"`
}

type AuditRecordSummary struct {
	ID        uint64    `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	RemoteIP  string `json:"remote_ip"`
	UserAgent string `json:"user_agent"`

	AuditTimestamp int64  `json:"audit_timestamp"`
	Signature      string `json:"signature"`

	EventType      string `json:"type"`
	EventTimestamp int64  `json:"timestamp"`

	RequestID  string `json:"request_id"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	DurationMs int64  `json:"duration_ms"`

	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	TokenID    int    `json:"token_id"`
	TokenName  string `json:"token_name"`
	UsingGroup string `json:"group"`

	ChannelID   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ChannelType int    `json:"channel_type"`
	Model       string `json:"model"`

	ContentType          string `json:"content_type"`
	RequestBody          string `json:"request_body"`
	RequestBodyEncoding  string `json:"request_body_encoding"`
	RequestBodyBytes     int    `json:"request_body_bytes"`
	RequestBodyTruncated bool   `json:"request_body_truncated"`
}

func (r AuditRecord) Summary() AuditRecordSummary {
	return AuditRecordSummary{
		ID:        r.ID,
		CreatedAt: r.CreatedAt,

		RemoteIP:  r.RemoteIP,
		UserAgent: r.UserAgent,

		AuditTimestamp: r.AuditTimestamp,
		Signature:      r.Signature,

		EventType:      r.EventType,
		EventTimestamp: r.EventTimestamp,

		RequestID:  r.RequestID,
		Method:     r.Method,
		Path:       r.Path,
		StatusCode: r.StatusCode,
		DurationMs: r.DurationMs,

		UserID:     r.UserID,
		Username:   r.Username,
		TokenID:    r.TokenID,
		TokenName:  r.TokenName,
		UsingGroup: r.UsingGroup,

		ChannelID:   r.ChannelID,
		ChannelName: r.ChannelName,
		ChannelType: r.ChannelType,
		Model:       r.Model,

		ContentType:          r.ContentType,
		RequestBody:          r.RequestBody,
		RequestBodyEncoding:  r.RequestBodyEncoding,
		RequestBodyBytes:     r.RequestBodyBytes,
		RequestBodyTruncated: r.RequestBodyTruncated,
	}
}
