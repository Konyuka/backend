package transfer

import (
	"database/sql"
	"fmt"
	"net/http"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"time"

	"github.com/gin-gonic/gin"
)

// LocalCloser -  transfer to an ingroup/agent direct
// for agent direct pass AGENTDIRECT as group
func LocalCloser(c *gin.Context) {

	var (
		err           error
		params        = new(transfer)
		phone         = new(models.Phone)
		tx            = call.DB.Begin()
		channel, conf interface{}
	)

	// 1. parse request
	if err = c.Bind(params); err != nil {
		call.Logger.Errorf("cannot parse transfer call request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("transfer to local closer : %+v\n", params)

	// 2. begin take call transaction
	err = func() error {

		// get phone detail
		err = call.DB.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// get conference extension
		err = tx.Raw(`
			SELECT conf_exten FROM vicidial_conferences 
			WHERE 
				extension = ? AND server_ip = ? LIMIT 1;`, fmt.Sprintf("SIP/%s", phone.Extension), phone.ServerIP,
		).Row().Scan(&conf)

		if err != nil {
			return fmt.Errorf("cannot find conference. %v", err)
		}

		// (A) fetch live channel from live_sip_channels
		err := tx.Raw(`
			SELECT channel FROM live_sip_channels 
			WHERE 
				server_ip = ? AND 
				extension = ? AND  
				channel_data = ?;`,
			phone.ServerIP, conf, fmt.Sprintf(`%d,Fmq`, conf),
		).Row().Scan(&channel)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot select live channels. %v", err)
		}

		if (err != nil && err == sql.ErrNoRows) || channel == nil {

			// (B) fetch live channel from auto_calls
			err = tx.Raw(`
				SELECT channel FROM vicidial_auto_calls 
				WHERE 
					server_ip = ? AND 
					phone_number = ? AND 
					(status = 'LIVE' OR  agent_grab = ?);`,
				phone.ServerIP, params.PhoneNumber, params.Username,
			).Row().Scan(&channel)

			if err != nil {
				return fmt.Errorf("cannot select live channel. %v", err)
			}
		}

		// update live agent
		err = tx.Exec(`UPDATE vicidial_live_agents SET external_recording='' where user = ?;`, params.Username).Error

		if err != nil {
			return fmt.Errorf("cannot update live agent. %v", err)
		}

		// stop monitor your channel
		channels := &models.VicidialManager{
			EntryDate: time.Now(),
			Status:    manager.NEW,
			Response:  manager.NO,
			ServerIP:  phone.ServerIP,
			Action:    "Hangup",
			CallerID:  fmt.Sprintf("RH12345%v0", time.Now().Unix()),
			CmdLineB:  fmt.Sprintf("Channel: %s", channel),
		}

		if err = channels.Save(tx); err != nil {
			return fmt.Errorf("cannot up live channels. %v", err)
		}

		// send request to closer ingroup
		var exten = ""

		if len(params.Agent) > 0 {
			/* AGENT */
			// Exten: 990009*AGENTDIRECT**368**729309658*duser2*duser*

			exten = fmt.Sprintf("990009*%v**%v**%v*%v*%v*", params.Group, params.LeadID, params.PhoneNumber, params.Username, params.Agent)
			/* XTERN BLIND */
			if len(params.Group) < 1 {
				exten = fmt.Sprintf("1%v", params.Agent)
			}

		} else if len(params.Group) > 0 {
			exten = fmt.Sprintf("990009*%v**%v**%v*%v**", params.Group, params.LeadID, params.PhoneNumber, params.Username)
		}

		if err = redirect(params, "XLvdcW", phone.ServerIP, exten); err != nil {
			return err
		}

		// add to call log
		err = tx.Exec(`
			INSERT INTO user_call_log 
				(user, call_date, call_type, server_ip, phone_number, number_dialed, lead_id, campaign_id) 
			VALUES
				(?, NOW(), 'BLIND_XFER', ?, ?, ?, ?, ?);
		`,
			params.Username, phone.ServerIP, params.PhoneNumber, "9"+params.PhoneNumber, params.LeadID, params.Campaign,
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert a new call log. %v", err)
		}

		// hangup other live channels
		rows, err := tx.Raw(`
			SELECT channel FROM live_sip_channels 
			WHERE server_ip = ? AND extension = ? AND channel LIKE ?;`,
			phone.ServerIP, fmt.Sprintf("%v", conf), "Local/%",
		).Rows()

		if err != nil {
			return fmt.Errorf("cannot select live channels. %v", err)
		}

		var hangups []string

		for rows.Next() {

			if err = rows.Scan(&channel); err != nil {
				return fmt.Errorf("cannot scan channel. %v", err)
			}

			hangups = append(hangups, fmt.Sprintf("%s", channel))
		}

		for i := range hangups {

			hang := &models.VicidialManager{
				EntryDate: time.Now(),
				Status:    manager.NEW,
				Response:  manager.NO,
				ServerIP:  phone.ServerIP,
				Action:    "Hangup",
				CallerID:  fmt.Sprintf("RH12345%v", time.Now().Unix()),
				CmdLineB:  fmt.Sprintf("Channel: %s", hangups[i]),
				CmdLineC: fmt.Sprintf(`
						Variable: ctuserserverconfleadphone=%v_%s_%s_%d_%s_%s`, 0, params.Username,
					phone.ServerIP, conf, params.LeadID, params.PhoneNumber,
				),
			}

			if err = hang.Save(tx); err != nil {
				return fmt.Errorf("cannot stop recording. %v", err)
			}
		}

		return nil
	}()

	// 3. error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[XFER LOCAL CLOSER] %v", err)
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
