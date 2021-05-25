package calls

import (
	"database/sql"
	"fmt"
	"net/http"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GrabParkedCall - unhold call
func GrabParkedCall(c *gin.Context) {

	var (
		err                error
		tx                 = call.DB.Begin()
		phone              = new(models.Phone)
		params             = new(grabcall)
		uid, channel, conf interface{}
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("could not parse request for parking call : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("grab parked call : %+v\n", params)

	// 2. begin call grabbing transaction
	err = func() error {

		// a. select phone details
		err = tx.Raw(`SELECT * FROM phones WHERE extension = ?;`, params.Phone).Scan(phone).Error

		if err != nil {
			return fmt.Errorf("cannot grab phone details. %v", err)
		}

		// b(i). fetch call channel
		err = tx.Raw(`SELECT uniqueid, channel FROM vicidial_manager WHERE callerid = ?;`, params.CallerID).Row().Scan(&uid, &channel)

		if err != nil && err == sql.ErrNoRows {

			// b. fetch call channel
			err = tx.Raw(`SELECT uniqueid, channel FROM vicidial_auto_calls WHERE auto_call_id = ?;`, params.CallerID).Row().Scan(&uid, &channel)

			if err != nil {
				return fmt.Errorf("cannot select call details from vicidial auto calls. %v", err)
			}
		}

		// c. insert into parked call recent
		tx.Exec(`
			INSERT INTO parked_channels_recent 
			SET
				server_ip = ?, 
				channel = ?,
				channel_group = ?,
				park_end_time = NOW();
		`, phone.ServerIP, channel, params.CallerID)

		// d. delete from parked channels
		err = tx.Exec(`DELETE FROM parked_channels where server_ip = ? and channel = ?;`, phone.ServerIP, params.CallerID).Error

		if err != nil {
			return fmt.Errorf("cannot delete from parked channel. %v", err)
		}

		// e. update parked channels to grabbed
		err = tx.Exec(`
			UPDATE park_log 
			SET 
				status = 'GRABBED',grab_time = NOW(),
				parked_sec = TIME_TO_SEC(TIMEDIFF(grab_time,parked_time))
			WHERE 
				uniqueid=? AND server_ip=? AND  extension=? 
			ORDER BY parked_time DESC LIMIT 1;`,
			uid, phone.ServerIP, params.CallerID,
		).Error

		if err != nil {
			return fmt.Errorf("cannot update parked log. %v", err)
		}

		// f. fetch conference extension
		err = tx.Raw(`
			SELECT conf_exten FROM vicidial_conferences 
			WHERE 
				extension = ? AND server_ip = ? LIMIT 1;`, fmt.Sprintf("SIP/%s", phone.Extension), phone.ServerIP,
		).Row().Scan(&conf)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot select conference. %v", err)
		}

		// g. grab call with manager
		var timestamp = time.Now().Unix()

		grab := &models.VicidialManager{
			EntryDate: time.Now(),
			Status:    manager.NEW,
			Response:  manager.NO,
			ServerIP:  phone.ServerIP,
			Action:    "Redirect",
			CallerID:  fmt.Sprintf("FPvdcW%v%v", timestamp, params.Username),
			CmdLineB:  fmt.Sprintf("Channel: %s", channel),
			CmdLineC:  "Context: default",
			CmdLineD:  fmt.Sprintf("Exten: %d", conf),
			CmdLineE:  "Priority: 1",
			CmdLineF:  fmt.Sprintf("CallerID: FPvdcW%v%v", timestamp, params.Username),
		}

		if err = grab.Save(tx); err != nil {
			return fmt.Errorf("cannot grab parked call. %v", err)
		}

		// h. update live agent
		err = tx.Exec(`
			UPDATE vicidial_live_agents SET status = 'INCALL' 
			WHERE user = ? AND campaign_id = ?;`, params.Username, params.Campaign,
		).Error

		return err
	}()

	// 3. error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[GRAB CALL] %v", err)
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

// ParkCall - hold call
func ParkCall(c *gin.Context) {

	var (
		err          error
		tx           = call.DB.Begin()
		params       = new(parkcall)
		phone        = new(models.Phone)
		channel, uid interface{}
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("could not parse request for parking call : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("park call : %+v\n", params)

	// 2. park call transaction
	err = func() error {

		// a. fetch phone details
		err = tx.Raw(`SELECT * FROM phones WHERE extension = ?;`, params.Phone).Scan(phone).Error

		if err != nil {
			return fmt.Errorf("cannot fetch phone details. %v", err)
		}

		// b(i). fetch call channel
		err = tx.Raw(`SELECT uniqueid, channel FROM vicidial_manager WHERE callerid = ?;`, params.CallerID).Row().Scan(&uid, &channel)

		if err != nil && err == sql.ErrNoRows {

			// b(ii). fetch call channel from vicidial_auto_calls
			err = tx.Raw(`SELECT uniqueid, channel FROM vicidial_auto_calls WHERE auto_call_id = ?;`, params.CallerID).Row().Scan(&uid, &channel)

			if err != nil {
				return fmt.Errorf("cannot select call details from manager. %v", err)
			}
		}

		// c. insert into parked channels
		err = tx.Exec(`
			INSERT INTO parked_channels VALUES (?, ?, ?, ?, ?, ?)`,
			channel, phone.ServerIP, params.CallerID, "park", "SIP/"+params.Phone, time.Now(),
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert into parked channels. %v", err)
		}

		// d. insert into park log
		err = tx.Exec(`
			INSERT INTO park_log 
			SET 
				uniqueid = ?, status = 'PARKED', 
				channel = ?, channel_group = ?,
				server_ip = ?, parked_time = NOW(),
				parked_sec = 0, extension = ?,
				user = ?, lead_id = ?;
		`, uid, channel, strings.ToUpper(params.Campaign), phone.ServerIP, params.CallerID,
			params.Username, params.LeadID,
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert into park log. %v", err)
		}

		// e. redirect to piano
		var timestamp = time.Now().Unix()

		hold := &models.VicidialManager{
			EntryDate: time.Now(),
			Status:    manager.NEW,
			Response:  manager.NO,
			ServerIP:  phone.ServerIP,
			Action:    "Redirect",
			CallerID:  fmt.Sprintf("LPvdcW%v%v", timestamp, params.Username),
			CmdLineB:  fmt.Sprintf("Channel: %s", channel),
			CmdLineC:  "Context: default",
			CmdLineD:  "Exten: " + phone.ParkOnExtension,
			CmdLineE:  "Priority: 1",
			CmdLineF:  fmt.Sprintf("CallerID: LPvdcW%v%v", timestamp, params.Username),
		}

		if err = hold.Save(tx); err != nil {
			return fmt.Errorf("cannot redirect parked call. %v", err)
		}

		return nil
	}()

	// 3. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[PARK CALL] %v", err)
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
