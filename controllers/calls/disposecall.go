package calls

import (
	"fmt"
	"net/http"
	"smartdial/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DisposeCall -
func DisposeCall(c *gin.Context) {

	var (
		err                  error
		params               = new(dispose)
		tx                   = call.DB.Begin()
		state, dialableLeads interface{}
		phone                = new(models.Phone)
		userGroup            interface{}
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse call disposition request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	err = func() error {

		// 2. phone details
		err = tx.Raw(`SELECT * FROM phones WHERE extension = ?`, params.Phone).Scan(phone).Error

		if err != nil {
			return fmt.Errorf("cannot select phone details. %v", err)
		}
		//  3. pick up user group
		err = tx.Raw(`SELECT user_group FROM vicidial_users WHERE  user = ?;`, params.Username).Row().Scan(&userGroup)

		if err != nil {
			return err
		}

		//update the vicidial_log table
		err = tx.Exec(`
			UPDATE vicidial_log
			SET
				status = ?
			WHERE
				uniqueid = ?;`,
			params.Status,
			params.UniqueID,
		).Error

		if err != nil {
			return fmt.Errorf("cannot update vicidial_log table. %v", err)
		}

		// 4. update agent log
		err = tx.Exec(`
			UPDATE vicidial_agent_log
			SET
				dispo_epoch = ?,
				talk_epoch = dispo_epoch,
				talk_sec = (talk_epoch - wait_epoch),
				status = ?,
				sub_status = ?
			WHERE user = ? AND agent_log_id = ?;
		`, time.Now().Unix(), params.Status, params.PauseCode, params.Username, GetAgentLogID(tx, params.Username)).Error

		if err != nil {
			return fmt.Errorf("cannot update agent log. %v", err)
		}

		// 5. new entry of agent_log
		err = tx.Exec(`
			INSERT INTO vicidial_agent_log
				(status,user, server_ip, event_time, campaign_id, pause_epoch, wait_sec, wait_epoch, user_group, sub_status, pause_type)
			VALUES
				(?,?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
		`, params.Status, params.Username, fmt.Sprintf("%s", phone.ServerIP), time.Now(), strings.ToUpper(params.Campaign),
			time.Now().Unix(), 0, time.Now().Unix(), fmt.Sprintf("%s", userGroup), params.PauseCode, "AGENT").Error

		if err != nil {
			return err
		}

		// 6. admin specificification for status after call
		err = tx.Raw(`
			SELECT pause_after_each_call FROM vicidial_campaigns WHERE campaign_id = ?;`, params.Campaign,
		).Row().Scan(&state)

		if err != nil {
			return fmt.Errorf("cannot select from campaigns. %v", err)
		}

		// 7. update vicidial list
		err = tx.Exec(`
			UPDATE vicidial_list
			SET
				status = ? ,
				modify_date = NOW()
			WHERE user = ? AND lead_id = ?;`,
			params.Status, params.Username, params.LeadID,
		).Error

		if err != nil {
			return fmt.Errorf("cannot update lead. %v", err)
		}

		// 8. OVERRIDE PAUSE CODE
		if len(params.PauseCode) > 1 {

			err = tx.Exec(`
				UPDATE vicidial_live_agents
				SET
					uniqueid = 0,
					status = 'PAUSED',
					callerid = '',
					lead_id = 0,
					external_hangup=0,
					external_status = '',
					comments = '',
					external_pause_code = '',
					pause_code = ?,
					agent_log_id = ?
				WHERE user = ? AND campaign_id = ? AND server_ip= ?;`, params.PauseCode,
				GetAgentLogID(tx, params.Username), params.Username,
				params.Campaign, phone.ServerIP,
			).Error

			if err != nil {
				return fmt.Errorf("cannot dispose call. %v", err)
			}

			// 9. NO PAUSE CODE BUT SHOULD PAUSE
		} else if len(params.PauseCode) < 1 && fmt.Sprintf("%s", state) == "Y" {

			err = tx.Exec(`
				UPDATE vicidial_live_agents
				SET
					status = 'PAUSED',
					callerid = '',
					lead_id = 0,
					external_hangup=0,
					external_status = '',
					comments = '',
					agent_log_id = ?,
					last_state_change = NOW()
				WHERE user = ? AND campaign_id = ? AND server_ip= ?;`, GetAgentLogID(tx, params.Username),
				params.Username, params.Campaign, phone.ServerIP,
			).Error

			if err != nil {
				return fmt.Errorf("cannot dispose call. %v", err)
			}

			// 10. ACTIVE
		} else {

			err = tx.Exec(`
				UPDATE vicidial_live_agents
				SET
					status = IF(outbound_autodial = "Y", "READY", "CLOSER"),
					callerid = '',
					channel = '',
					lead_id = 0,
					external_hangup = 0,
					external_status = '',
					comments = '',
					external_recording = '',
					last_state_change = NOW(),
					agent_log_id = ?
				WHERE user = ? AND campaign_id = ? AND server_ip = ?;`, GetAgentLogID(tx, params.Username),
				params.Username, params.Campaign, phone.ServerIP,
			).Error

			if err != nil {
				return fmt.Errorf("cannot dispose call. %v", err)
			}
		}

		// save callback
		if len(params.CallbackTime) > 0 {

			d, _ := time.Parse(time.RFC3339, params.CallbackTime)

			// fetch the list id
			var listID interface{}

			err = tx.Raw(`SELECT list_id FROM vicidial_list WHERE lead_id = ?;`, params.LeadID).Row().Scan(&listID)

			if err != nil {
				return err
			}

			// add a callback
			err = tx.Exec(`
				INSERT INTO vicidial_callbacks
				(
					lead_id, list_id, campaign_id, status, entry_time, callback_time,
					user, recipient, comments, user_group, lead_status, customer_time
				)
				VALUES
					(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, params.LeadID, listID, params.Campaign, "ACTIVE", time.Now(), d.Format("2006-01-02 15:04:00"),
				params.Username, params.Recipient, params.Comment, userGroup, params.PauseCode, time.Now(),
			).Error

			if err != nil {
				return err
			}
		}

		//if the call  is inbound
		if params.Type == "INBOUND" {

			err = tx.Exec(`
			UPDATE
				vicidial_closer_log
			SET
				status=?
			WHERE
			 lead_id=? AND
				user=?;
		   `,
				params.Status,
				params.LeadID,
				params.Username,
			).Error
		}

		//if the call  is auto(sent from dialer) //AND uniqueid = '$uniqueid'; ";
		if params.Type == "AUTO" {

			// fetch the list id
			var listID interface{}
			var userGroup interface{}

			err = tx.Raw(`SELECT list_id FROM vicidial_list WHERE lead_id = ?;`, params.LeadID).Row().Scan(&listID)

			err = tx.Raw(`SELECT user_group FROM vicidial_users where user= ?;`, params.Username).Row().Scan(&userGroup)

			err = tx.Exec(`
				UPDATE
					vicidial_log
				SET
					user        =?,
					comments    ='AUTO',
					list_id     =?,
					status      =?,
					user_group  =?,
					alt_dial = 'MAIN'
				WHERE
					lead_id=?
			`,
				params.Username,
				listID,
				params.Status,
				userGroup,
				params.LeadID,
			).Error
		}

		//get the leads
		err = tx.Raw(`SELECT COUNT(*) FROM vicidial_list vl INNER JOIN vicidial_lists vls ON vl.list_id = vls.list_id WHERE vls.campaign_id = ? AND vl.status = 'NEW';`, params.Campaign).Row().Scan(&dialableLeads)

		return nil
	}()

	// 11. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[DISPOSE CALL] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 12. success
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusText(http.StatusOK),
		"leads":  dialableLeads,
	})
	c.Abort()
	return
}
