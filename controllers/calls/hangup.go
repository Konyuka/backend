package calls

import (
	"database/sql"
	"fmt"
	"net/http"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"time"

	"github.com/gin-gonic/gin"
)

// HangUpCall -
func HangUpCall(c *gin.Context) {

	var (
		err     error
		params  = new(hangup)
		phone   = new(models.Phone)
		tx      = call.DB.Begin()
		channel interface{}
		conf    interface{}
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse hangup request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	err = func() error {

		// 2. fetch phone details
		if err = tx.Raw(`SELECT * FROM phones WHERE extension = ?;`, params.Phone).Scan(&phone).Error; err != nil {
			return fmt.Errorf("cannot find phone details for %v. %v", params.Phone, err)
		}

		// 3. fetch conference extension
		err = tx.Raw(`
			SELECT conf_exten FROM vicidial_conferences
			WHERE
				extension = ? AND server_ip = ? LIMIT 1;`, fmt.Sprintf("SIP/%s", phone.Extension), phone.ServerIP,
		).Row().Scan(&conf)

		if err != nil {
			return fmt.Errorf("cannot find conference. %v", err)
		}

		// 3. hang up recording
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

		// 4. hangup live channel
		var liveChan interface{}
		var serverIp string

		err = tx.Raw(`SELECT channel,server_ip FROM vicidial_manager WHERE callerid = ?;`, params.CallerID).Row().Scan(&liveChan, &serverIp)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot select channel from manager. %v", err)
		}

		// check from vicidial_auto_calls - incase the call was picked from queue
		if liveChan == nil || err == sql.ErrNoRows {

			err = tx.Raw(`SELECT channel,server_ip FROM vicidial_auto_calls WHERE auto_call_id = ?;`, params.CallerID).Row().Scan(&liveChan, &serverIp)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// incase the call was an auto-call
			if liveChan == nil || err == sql.ErrNoRows {

				err = tx.Raw(`
					SELECT channel,server_ip FROM vicidial_live_agents WHERE user = ? AND callerid = ?;`,
					params.Username, params.CallerID,
				).Row().Scan(&liveChan, &serverIp)

				if err != nil && err != sql.ErrNoRows {
					return err
				}
			}
		}

		if liveChan != nil {

			live := &models.VicidialManager{
				EntryDate: time.Now(),
				Status:    manager.NEW,
				Response:  manager.NO,
				ServerIP:  serverIp, //"172.16.10.203", //phone.ServerIP,
				Action:    "Hangup",
				CallerID:  fmt.Sprintf("HLvdcW%v%v", time.Now().Unix(), params.Username),
				CmdLineB:  fmt.Sprintf("Channel: %s", liveChan),
			}

			if err = live.Save(tx); err != nil {
				return fmt.Errorf("cannot hangup live channels. %v", err)
			}
		}

		// // 5. other live channels
		// channels := &models.VicidialManager{
		// 	EntryDate: time.Now(),
		// 	Status:    manager.NEW,
		// 	Response:  manager.NO,
		// 	ServerIP:  phone.ServerIP,
		// 	Action:    "Hangup",
		// 	CallerID:  fmt.Sprintf("RH12345%v", time.Now().Unix()),
		// 	CmdLineB:  fmt.Sprintf("Channel: %s", hangups[0]),
		// }

		// if err = channels.Save(tx); err != nil {
		// 	return fmt.Errorf("cannot up live channels. %v", err)
		// }
		//fmt.Println(phone.ServerIP)

		// 6. hangup recordings
		for i := range hangups {

			recording := &models.VicidialManager{
				EntryDate: time.Now(),
				Status:    manager.NEW,
				Response:  manager.NO,
				ServerIP:  serverIp, //phone.ServerIP,
				Action:    "Hangup",
				CallerID:  fmt.Sprintf("CH123456%v", time.Now().Unix()),
				CmdLineB:  fmt.Sprintf("Channel: %s", hangups[i]),
				CmdLineC: fmt.Sprintf(`
						Variable: ctuserserverconfleadphone=%v_%s_%s_%d_%s_%s`, 0, params.Username,
					serverIp, conf, params.LeadID, params.PhoneNumber,
				),
			}

			if err = recording.Save(tx); err != nil {
				return fmt.Errorf("cannot stop recording. %v", err)
			}
		}

		// 7. dispo mode - set to 'paused' or 'ready'
		err = tx.Exec(`
			UPDATE vicidial_live_agents
			SET
				callerid = '',
				status = 'PAUSED',
				comments = '',
				last_state_change = NOW()
			WHERE user = ? AND campaign_id = ? AND server_ip = ?;`,
			params.Username, params.Campaign, serverIp,
		).Error

		if err != nil {
			return fmt.Errorf("cannot dispose call. %v", err)
		}

		return nil
	}()

	// 9. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[HANGUP CALL] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 10. success
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusText(http.StatusOK),
	})
	c.Abort()
	return
}
