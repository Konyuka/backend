package calls

import (
	"database/sql"
	"fmt"
	"net/http"
	"smartdial/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SwitchPauseCode -
func SwitchPauseCode(c *gin.Context) {

	var (
		err       error
		tx        = call.DB.Begin()
		params    = new(changepause)
		phone     = new(models.Phone)
		userGroup interface{}
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("could not parse request for switching pause codes : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("switch pause code : %+v\n", params)

	// begin transaction for switching pause codes
	err = func() error {

		var pauseEpoch, waitEpoch int

		// 2. grab phone details
		err = tx.Raw(`SELECT * FROM phones WHERE extension = ?`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find phone details. %v", err)
		}

		//  3. pick up phone_login
		err = tx.Raw(`SELECT user_group FROM vicidial_users WHERE  user = ?;`, params.Username).Row().Scan(&userGroup)

		if err != nil {
			return err
		}

		err = tx.Raw(`SELECT pause_epoch from vicidial_agent_log WHERE agent_log_id = ? AND user = ?;`, time.Now().Unix(), GetAgentLogID(tx, params.Username), params.Username).Row().Scan(&pauseEpoch)

		if err != nil {
			return err
		}

		// 3. update previous log entry
		err = tx.Exec(`UPDATE vicidial_agent_log SET wait_epoch = ? WHERE agent_log_id = ? AND user = ?;`, time.Now().Unix(), GetAgentLogID(tx, params.Username), params.Username).Error

		if err != nil {
			return err
		}

		//pause_sec = (wait_epoch - pause_epoch)

		//err = tx.Raw(`SELECT wait_epoch from vicidial_agent_log where agent_log_id =?` ,GetAgentLogID(tx, params.Username)).Row().Scan(&pauseEpoch)
		err = tx.Raw(`SELECT wait_epoch from vicidial_agent_log WHERE agent_log_id = ? AND user = ?;`, time.Now().Unix(), GetAgentLogID(tx, params.Username), params.Username).Row().Scan(&waitEpoch)

		if err != nil {
			return err
		}

		//waitEpoch

		pauseEpoch = waitEpoch - pauseEpoch

		err = tx.Exec(`UPDATE vicidial_agent_log SET pause_sec = ? WHERE agent_log_id = ? AND user = ?;`, pauseEpoch, GetAgentLogID(tx, params.Username), params.Username).Error

		if err != nil {
			return err
		}

		// 4. add log entry for agent
		err = tx.Exec(`INSERT INTO vicidial_agent_log (user, server_ip, event_time, campaign_id, pause_epoch, wait_sec, wait_epoch, user_group, sub_status, pause_type) VALUES (?, ?, ?, ?, ?, ?, ?)`, params.Username, fmt.Sprintf("%s", phone.ServerIP), time.Now(), strings.ToUpper(params.Campaign), time.Now().Unix(), 0, time.Now().Unix(), fmt.Sprintf("%s", userGroup), params.PauseCode, "AGENT").Error

		if err != nil {
			return err
		}

		// 5. update live agent
		gor := tx.Exec(`UPDATE vicidial_live_agents SET pause_code = ? , agent_log_id = ? WHERE user = ? AND extension = ? AND server_ip = ? AND status = 'PAUSED' AND pause_code IS NOT NULL;`, params.PauseCode, GetAgentLogID(tx, params.Username), params.Username, fmt.Sprintf("SIP/%v", params.Phone), phone.ServerIP)

		if err != nil || gor.RowsAffected < 1 {
			return fmt.Errorf("cannot update pause code. %v", err)
		}

		return nil
	}()

	// 6. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 7. success
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusText(http.StatusOK),
	})
	c.Abort()
	return
}

// TogglePause -
func TogglePause(c *gin.Context) {

	var (
		err    error
		params = new(togglepause)
		tx     = call.DB.Begin()

		serverIP, userGroup interface{}
		pauseEpoch          = time.Now().Unix()
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse toggle pause request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("toggle pause : %+v\n", params)

	// 2. begin a transaction
	err = func() error {
		fmt.Println("1")

		var agentLogID = GetAgentLogID(tx, params.Username)

		// 1. fetch usergroup
		err = tx.Raw(`SELECT user_group FROM vicidial_users WHERE  user = ?;`, params.Username).Row().Scan(&userGroup)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		//  2. get server ip
		if err = tx.Raw(`SELECT server_ip FROM phones WHERE extension = ?`, params.Phone).Row().Scan(&serverIP); err != nil {
			return fmt.Errorf("cannot find server ip.%v", err)
		}

		//  4. insert agent log
		var status string
		var pauseSec int64

		// READY
		if strings.ToLower(params.State) == "ready" {

			// fetch autodial blended value
			var blended interface{}

			err = tx.Raw(`
				SELECT outbound_autodial FROM vicidial_live_agents WHERE user = ? AND campaign_id = ?;`,
				params.Username, params.Campaign,
			).Row().Scan(&blended)

			if err != nil {
				return err
			}

			if blended != nil && fmt.Sprintf("%s", blended) == "Y" {
				status = "READY"
			}

			if (blended != nil && fmt.Sprintf("%s", blended) == "N") || len(status) < 1 {
				status = "CLOSER"
			}


			err = tx.Raw(`SELECT pause_epoch from vicidial_agent_log WHERE agent_log_id = ? AND user = ?;`, GetAgentLogID(tx, params.Username), params.Username).Row().Scan(&pauseEpoch) //time.Now().Unix(),

			if err != nil {
				return err
			}
			fmt.Println("2")

			pauseSec = time.Now().Unix() - pauseEpoch


			err = tx.Exec(`UPDATE vicidial_agent_log SET wait_sec = UNIX_TIMESTAMP(NOW()) , pause_sec = ? WHERE user = ? AND campaign_id = ? AND agent_log_id = ? ORDER BY event_time DESC LIMIT 1;`, pauseSec, params.Username, params.Campaign, agentLogID).Error

			if err != nil {
				return err
			}
			fmt.Println("3")

			// update agent log
			// err = tx.Exec(`
			// 	UPDATE vicidial_agent_log
			// 	SET
			// 		wait_epoch = UNIX_TIMESTAMP(NOW()),
			// 		pause_sec = (wait_epoch - pause_epoch)
			// 	WHERE
			// 		user = ? AND campaign_id = ? AND agent_log_id = ?;`,
			// 	params.Username, params.Campaign, agentLogID,
			// ).Error

			// if err != nil {
			// 	return err
			// }

			// update agent log id for live agent
			err = tx.Exec(`
				UPDATE vicidial_live_agents
				SET
					status = ?,
					lead_id = 0,
					uniqueid = '0',
					last_update_time = NOW(),
					agent_log_id = ?,
					preview_lead_id = 0,
					external_lead_id = 0,
					pause_code='',
					external_pause_code='',
					last_state_change = NOW()
				WHERE
					user = ? AND
					campaign_id = ?;`,
				status, agentLogID,
				params.Username, params.Campaign,
			).Error

			if err != nil {
				return fmt.Errorf("cannot update live agent. %v", err)
			}

			// PAUSED
		}

		if strings.ToLower(params.State) == "paused" && len(params.PauseCode) > 0 {

			err = tx.Raw(`SELECT pause_epoch from vicidial_agent_log WHERE agent_log_id = ? AND user = ?;`, GetAgentLogID(tx, params.Username), params.Username).Row().Scan(&pauseEpoch) //time.Now().Unix(),

			if err != nil {
				return err
			}
			fmt.Println("2")

			pauseSec = time.Now().Unix() - pauseEpoch

			err = tx.Exec(`UPDATE vicidial_agent_log SET wait_sec = UNIX_TIMESTAMP(NOW()) - wait_epoch, pause_sec = ? WHERE user = ? AND campaign_id = ? AND agent_log_id = ? ORDER BY event_time DESC LIMIT 1;`, pauseSec, params.Username, params.Campaign, agentLogID).Error

			if err != nil {
				return err
			}
			fmt.Println("3")

			// update previous agent log
			// err = tx.Exec(`UPDATE vicidial_agent_log SET wait_sec = UNIX_TIMESTAMP(NOW()) - wait_epoch, pause_sec = (wait_epoch - pause_epoch)WHERE user = ? AND campaign_id = ? AND agent_log_id = ? ORDER BY event_time DESC LIMIT 1;`,params.Username, params.Campaign, agentLogID,).Error

			// if err != nil {
			// 	return err
			// }

			// now insert a new agent log record
			tx.Exec(`INSERT INTO vicidial_agent_log (user, server_ip, event_time, campaign_id, pause_epoch, wait_sec, wait_epoch, user_group, sub_status, pause_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, params.Username, fmt.Sprintf("%s", serverIP), time.Now(), strings.ToUpper(params.Campaign), pauseEpoch, 0, pauseEpoch, fmt.Sprintf("%s", userGroup), params.PauseCode, "AGENT")

			fmt.Println("4")
			// update agent log id for live agent
			err = tx.Exec(` UPDATE vicidial_live_agents SET status = ?,agent_log_id = ?,pause_code = ? WHERE user = ? AND campaign_id = ?;`, strings.ToUpper(params.State), agentLogID, params.PauseCode, params.Username, params.Campaign).Error

			if err != nil {
				return fmt.Errorf("cannot update live agent. %v", err)
			}
		}

		return nil
	}()

	// 3. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[TOGGLE PAUSE/READY] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 4. success
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusText(http.StatusOK),
	})
	c.Abort()
	return
}
