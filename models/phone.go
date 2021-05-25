package models

// Phone -
type Phone struct {
	Extension         string `json:"extension"`
	DialNumber        string `json:"dial_number"`
	VoiceMailID       string `json:"voicemail_id"`
	PhoneIP           string `json:"phone_ip"`
	ComputerIP        string `json:"computer_ip"`
	ServerIP          string `json:"server_ip"`
	Login             string `json:"login"`
	Pass              string `json:"pass"`
	ConfExten         string `json:"conf_exten"`
	ExtContext        string `json:"ext_context"`
	PhoneRingTimeout  int    `json:"phone_ring_timeout"`
	OnHookAgent       string `json:"on_hook_agent"`
	ParkOnExtension   string `json:"park_on_extension"`
	DTMFSendExtension string `json:"dtmf_send_extension"`
}
