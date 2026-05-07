package messaging

import "encoding/json"

const (
	TaskExchange           = "task.events"
	UserExchange           = "user.events"
	CreditResultExchange   = "credit.results"
	CaptchaExchange        = "captcha.events"
	QueueAuditCaptchaEvents = "audit.captcha.events"
)

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
