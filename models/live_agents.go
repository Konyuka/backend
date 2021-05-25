package models

import (
	"fmt"
	"smartdial/data"
	"time"
)

// LiveAgent -
type LiveAgent struct {
	LiveAgentID    int
	User           string
	ServerIP       string
	ConfExten      string
	Extension      string
	Status         string
	LeadID         string
	CampaignID     string
	RandomID       string
	LastCallTime   time.Time
	LastUpdateTime time.Time
	LastCallFinish time.Time
	//
	CloserCampaigns string
	UserLevel       int
	//

}

// Save - newly logged in agent
func (l LiveAgent) Save() error {

	var db = data.GetDB()

	query := fmt.Sprintf(`
		INSERT INTO vicidial_live_agents 
			(live_agent_id, user, server_ip, conf_exten, extension, status, lead_id, campaign_id, uniqueid, callerid, channel, random_id, 
			last_call_time, last_update_time, last_call_finish, closer_campaigns, call_server_ip, user_level, 
			comments, campaign_weight, calls_today, external_hangup, external_status, external_pause, external_dial, 
			external_ingroups, external_blended, external_igb_set_user, external_update_fields, external_update_fields_data, 
			external_timer_action, external_timer_action_message, external_timer_action_seconds, agent_log_id, last_state_change, 
			agent_territories, outbound_autodial, manager_ingroup_set, ra_user, ra_extension, external_dtmf, external_transferconf, 
			external_park, external_timer_action_destination, on_hook_agent, on_hook_ring_time, ring_callerid, last_inbound_call_time, 
			last_inbound_call_finish, campaign_grade, external_recording, external_pause_code, pause_code, preview_lead_id, external_lead_id, 
			last_inbound_call_time_filtered, last_inbound_call_finish_filtered) 
		VALUES 
			(NULL, 'duser2', '172.16.10.209', '8600051', 'SIP/102', 'PAUSED', '0', 'DCAMP', '', '', '', '11036487', 
			'2020-08-19 16:29:09', '2020-08-19 16:30:03', '2020-08-19 16:29:09', '-', NULL, '1', 
			NULL, '0', '0', '', '', '', '', 
			NULL, '0', '', '0', '', 
			'', '', '-1', '196', '2020-08-19 16:29:14', NULL, 'N', 'N', '', '', '', '', '', '', 'N', '60', '', '2020-08-19 16:29:09', '2020-08-19 16:29:09', '10', '', '', 'LOGIN', '0', '0', '2020-08-19 16:29:09', '2020-08-19 16:29:09')
	`)

	return db.Exec(query).Error
}
