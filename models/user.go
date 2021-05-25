package models

import "time"

// User struct
type User struct {
	UserID           uint64 `json:"user_id"`
	User             string `json:"user"`
	Pass             string `json:"pass"`
	FullName         string `json:"full_name"`
	UserLevel        int    `json:"user_level"`
	PhoneLogin       string
	PhonePass        string
	FailedLoginCount int
	LastLoginDate    time.Time
}
