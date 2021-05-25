package models

import "time"

// CarrierLog -
type CarrierLog struct {
	UniqueID        string    `json:"uniqueid"`
	CallDate        time.Time `json:"call_date"`
	ServerIP        string    `json:"server_ip"`
	LeadID          int       `json:"lead_id"`
	HangupCause     int       `json:"hangup_cause"`
	DialStatus      string    `json:"dialstatus"`
	Channel         string    `json:"channel"`
	DialTime        int       `json:"dial_time"`
	AnsweredTime    int       `json:"answered_time"`
	SIPHangupCause  int       `json:"sip_hangup_cause"`
	SIPHangupReason string    `json:"sip_hangup_reason"`
	CallerCode      string    `json:"caller_code"`
}
