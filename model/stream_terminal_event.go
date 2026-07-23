package model

import (
	"gorm.io/gorm/clause"
)

// StreamTerminalEvent persists the final transport outcome of one streamed
// Responses request independently from usage settlement and user-facing logs.
// A failed/refunded request must still leave an operational record here.
type StreamTerminalEvent struct {
	Id                       int    `json:"id" gorm:"primaryKey"`
	RequestId                string `json:"request_id" gorm:"type:varchar(64);uniqueIndex:idx_stream_terminal_request_id"`
	IngressRequestId         string `json:"ingress_request_id" gorm:"type:varchar(64);index:idx_stream_terminal_ingress_request_id"`
	UpstreamRequestId        string `json:"upstream_request_id" gorm:"type:varchar(128);index"`
	UpstreamResponseId       string `json:"upstream_response_id" gorm:"type:varchar(128);index"`
	CreatedAt                int64  `json:"created_at" gorm:"bigint;index:idx_stream_terminal_created_at"`
	StartedAt                int64  `json:"started_at" gorm:"bigint;index:idx_stream_terminal_affinity_started,priority:2"`
	DurationMs               int64  `json:"duration_ms" gorm:"bigint;default:0"`
	UserId                   int    `json:"user_id" gorm:"index:idx_stream_terminal_user_created,priority:1"`
	TokenId                  int    `json:"token_id" gorm:"index"`
	ChannelId                int    `json:"channel_id" gorm:"index:idx_stream_terminal_channel_created,priority:1"`
	ModelName                string `json:"model_name" gorm:"size:128;index"`
	Group                    string `json:"group" gorm:"column:group;size:64"`
	AffinityRuleName         string `json:"affinity_rule_name" gorm:"size:128"`
	AffinityKeyFp            string `json:"affinity_key_fp" gorm:"size:40;index:idx_stream_terminal_affinity_started,priority:1"`
	RequestHost              string `json:"request_host" gorm:"size:255;index"`
	RequestPath              string `json:"request_path" gorm:"size:255;index"`
	TerminalStatus           string `json:"terminal_status" gorm:"size:32;index:idx_stream_terminal_status_created,priority:1"`
	EndReason                string `json:"end_reason" gorm:"size:64;index"`
	EndError                 string `json:"end_error" gorm:"type:text"`
	ErrorType                string `json:"error_type" gorm:"size:64"`
	ErrorCode                string `json:"error_code" gorm:"size:128;index"`
	FailureSource            string `json:"failure_source" gorm:"size:32;index"`
	UpstreamTerminalEvent    string `json:"upstream_terminal_event" gorm:"size:64;index"`
	UpstreamHttpStatus       int    `json:"upstream_http_status" gorm:"default:0"`
	UpstreamResponseStatus   string `json:"upstream_response_status" gorm:"size:32;index"`
	UpstreamErrorCode        string `json:"upstream_error_code" gorm:"size:128;index"`
	UpstreamErrorMessage     string `json:"upstream_error_message" gorm:"type:text"`
	IncompleteReason         string `json:"incomplete_reason" gorm:"size:128;index"`
	UpstreamRequestBodyBytes int64  `json:"upstream_request_body_bytes" gorm:"bigint;default:0"`
	HttpStatus               int    `json:"http_status" gorm:"default:0"`
	IntendedStatus           int    `json:"intended_status" gorm:"default:0"`
	ResponseBytes            int64  `json:"response_bytes" gorm:"bigint;default:0"`
	ReceivedEvents           int    `json:"received_events" gorm:"default:0"`
	SoftErrorCount           int    `json:"soft_error_count" gorm:"default:0"`
	ResponseCompleted        bool   `json:"response_completed" gorm:"default:false;index"`
	ClientGone               bool   `json:"client_gone" gorm:"default:false;index"`
	BillingSource            string `json:"billing_source" gorm:"size:32"`
	BillingState             string `json:"billing_state" gorm:"size:32;index"`
	PreConsumedQuota         int    `json:"pre_consumed_quota" gorm:"default:0"`
	UsedChannels             string `json:"used_channels" gorm:"type:text"`
}

func (StreamTerminalEvent) TableName() string {
	return "stream_terminal_events"
}

// RecordStreamTerminalEvent is idempotent by request_id. The relay controller
// has several deferred cleanup paths, so a duplicate attempt must not create a
// second terminal row.
func RecordStreamTerminalEvent(event *StreamTerminalEvent) error {
	if event == nil || event.RequestId == "" {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "request_id"}},
		DoNothing: true,
	}).Create(event).Error
}
