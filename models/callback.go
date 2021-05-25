package models

import (
	"time"
)

// VicidialCallback -
type VicidialCallback struct {
	PhoneNumber  string
	CallbackID   int64 `json:"CallbackID"`
	LeadID       int64
	CampaignID   string
	EntryTime    time.Time `json:"callback_time"`
	CallbackTime time.Time
	Comments     string
	CustomerTime *time.Time
}
