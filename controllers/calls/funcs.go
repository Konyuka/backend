package calls

import (
	"database/sql"
	"errors"
	"fmt"
	"smartdial/utils"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

// AllowedInboundGroups -
func AllowedInboundGroups(tx *gorm.DB, campaign string) ([]string, error) {

	var groups interface{}

	err := tx.Raw(`SELECT closer_campaigns FROM vicidial_campaigns WHERE campaign_id = ?;`, campaign).Row().Scan(&groups)

	if err != nil {
		return nil, errors.New("no inbound groups for the selected campaign")
	}

	var temp = fmt.Sprintf("%s", groups)

	temp = strings.TrimSpace(temp[:strings.LastIndex(temp, "-")])

	return strings.Split(temp, " "), nil
}

// MyGroups - groups the user can subscribe to
func MyGroups(tx *gorm.DB, cid string, selected []string) (map[string]bool, error) {

	var (
		err error
		res = make(map[string]bool)
	)

	// fetch allowed inbound groups
	groups, err := AllowedInboundGroups(tx, cid)

	if err != nil {
		return res, err
	}

	// prepare selected ingroups without agent direct
	for _, item := range selected {
		if item != "AGENTDIRECT" {
			res[item] = true
		}
	}

	if len(selected) > 0 {

		for _, val := range groups {
			if !utils.Find(selected, val) && val != "AGENTDIRECT" {
				res[val] = false
			}
		}

	} else {

		// return the list without AGENTDIRECT
		for _, val := range groups {

			if val != "AGENTDIRECT" {

				res[val] = false
			}
		}
	}

	return res, err
}

// AddInbound -
func AddInbound(tx *gorm.DB, username string, groups []string) error {

	var err error

	// 1.delete existing records first
	if err = tx.Exec("DELETE FROM vicidial_live_inbound_agents WHERE user = ?;", username).Error; err != nil {
		return fmt.Errorf("cannot delete from live inbound agents table. %v", err)
	}

	for _, group := range groups {

		// 2. values from inbound group agents
		var groupW, callsT, groupG, callsTF interface{}

		err = tx.Raw(`
			SELECT group_weight,calls_today,group_grade,calls_today_filtered
			FROM vicidial_inbound_group_agents
			WHERE user = ? AND group_id = ?;`,
			username, group,
		).Row().Scan(&groupW, &callsT, &groupG, &callsTF)

		if groupW == nil {
			groupW = 0
		}

		if groupG == nil {
			groupG = 0
		}

		if callsT == nil {
			callsT = 0
		}

		if callsTF == nil {
			callsTF = 0
		}

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("unable to select from inbound group agents. %v", err)
		}

		// add new live inbound agents
		err = tx.Exec(`
			INSERT INTO vicidial_live_inbound_agents
				(user,group_id,group_weight,calls_today,last_call_time, last_call_finish,
				last_call_time_filtered,last_call_finish_filtered,group_grade,calls_today_filtered)
			VALUES
				(?,?,?,?,?,?,
				?,?,?,?);`,
			username, group, fmt.Sprint(groupW), fmt.Sprintf("%v", callsT), time.Now(), time.Now(),
			time.Now(), time.Now(), fmt.Sprintf("%v", groupG), fmt.Sprintf("%v", callsTF),
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert into live inbound agents %v", err)
		}
	}

	return err
}

// GetAgentLogID -
func GetAgentLogID(tx *gorm.DB, username string) string {

	var agentLID interface{}

	err := tx.Raw(`
		SELECT agent_log_id FROM vicidial_agent_log WHERE user = ?
		ORDER BY agent_log_id DESC LIMIT 1;`,
		username,
	).Row().Scan(&agentLID)

	if err != nil {
		return ""
	}

	return fmt.Sprintf("%v", agentLID)
}

// GetConference -
func GetConference(tx *gorm.DB, serverIP, sipExt string) (interface{}, error) {

	var (
		err  error
		conf interface{}
	)

	err = tx.Raw(`
		SELECT conf_exten FROM vicidial_conferences WHERE extension = ? AND leave_3way = '0' LIMIT 1;`, sipExt,
	).Row().Scan(&conf)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if conf == nil || err == sql.ErrNoRows {

		// reserve a conf extension
		err = tx.Exec(`
			UPDATE vicidial_conferences
			SET
				extension = ?
			WHERE
				server_ip = ? AND
				(extension = '' OR extension IS NULL) AND
				leave_3way = '0'
			LIMIT 1;`, sipExt, serverIP,
		).Error

		if err != nil {
			return nil, fmt.Errorf("unable to allocate new conference. %v", err)
		}

		// select reserved conf extension
		err = tx.Raw(`
			SELECT conf_exten FROM vicidial_conferences
			WHERE
				extension = ? AND server_ip = ? LIMIT 1;`, sipExt, serverIP,
		).Row().Scan(&conf)

		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		if conf == nil || err == sql.ErrNoRows {
			return nil, errors.New("cannot grab a conference")
		}
	}

	return conf, nil
}

// GetScriptURL - returns iframe url
func GetScriptURL(tx *gorm.DB, inout, owner, campaign string) (string, error) {

	var (
		err error

		link string

		script, scriptContent interface{}
	)

	err = func() error {

		// script for incoming calls for a particular ingroup
		if strings.ToUpper(inout) == "IN" {

			err = tx.Raw(`
				SELECT ingroup_script FROM vicidial_inbound_groups
				WHERE
				group_id = ? AND
					get_call_launch = 'SCRIPT';`, campaign,
			).Row().Scan(&script)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// script for incoming calls for a campaign
		} else if strings.ToUpper(inout) == "OUT" {

			err = tx.Raw(`
				SELECT campaign_script FROM vicidial_campaigns
				WHERE
					campaign_id = ? AND
					get_call_launch = 'SCRIPT'
			`, campaign).Row().Scan(&script)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

		}

		// if script is NOT found
		if script != nil {

			err = tx.Raw(`
					SELECT script_text FROM vicidial_scripts WHERE script_id = ?`, script,
			).Row().Scan(&scriptContent)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			for _, k := range strings.Split(fmt.Sprintf("%s", scriptContent), " ") {
				if strings.Contains(k, "http") {
					link = strings.Split(k, `"`)[1]
				}
			}

			// all script variables
			var variables = []string{
				"--A--phone_number--B--",
				"--A--owner--B--",
			}

			for _, v := range variables {
				link = strings.Replace(link, v, owner, 1)
			}
		}

		return nil
	}()

	return link, err
}
