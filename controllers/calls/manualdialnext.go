package calls

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"math/rand"
	"net/http"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"strconv"
	"time"
)

//create a global variable to keep the uniqueid
var uniqueIDVar string

//Get a unique ID  for a call
func uniqueID() {
	rand.Seed(time.Now().UnixNano())
	final := strconv.FormatInt(time.Now().Unix(), 10) + "." + strconv.Itoa(rand.Intn(9999-1)+1)
	uniqueIDVar = final
}

//get the epoch time start stop
func epochTime() string {
	rand.Seed(time.Now().UnixNano())
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// DialNext -
func DialNext(c *gin.Context) {

	var (
		err             error
		tx              = call.DB.Begin()
		params          = new(dialnext)
		leadID, listID  interface{}
		phoneNumber     interface{}
		MqueryCID, link string
		timestamp       = time.Now().Format("0102150405")
	)

	// 1. parse request
	if err = c.Bind(params); err != nil {
		call.Logger.Errorf("cannot parse dial next request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("dial next : %+v\n", params)

	// transaction -
	err = func() error {

		// 1. fetch phone about user
		var phone = new(models.Phone)

		err = tx.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// 2. fetch list id from lists
		err = tx.Raw(`SELECT list_id FROM vicidial_lists WHERE campaign_id = ?;`, params.Campaign).Row().Scan(&listID)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot select list id for campaign. %v", err)
		}

		if err == sql.ErrNoRows {
			return errors.New("no list for this campaign")
		}

		// 4. select from vicidial_hopper
		err = tx.Raw(`
			SELECT lead_id FROM vicidial_hopper WHERE campaign_id = ? AND list_id = ? ORDER BY hopper_id ASC LIMIT 1;`,
			params.Campaign, listID,
		).Row().Scan(&leadID)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// 5. delete from hopper
		err = tx.Exec(`DELETE FROM vicidial_hopper  WHERE lead_id = ?;`, leadID).Error

		if err != nil {
			return fmt.Errorf("cannot delete from hopper. %v", err)
		}

		// 5. check in leads & hopper. update if exists

		//repeating the above functionality,,, use the hopper to avoid over loading
		//
		// if leadID == nil {

		// 	err = tx.Raw(`SELECT lead_id FROM vicidial_list WHERE list_id = ? AND status = 'NEW' ORDER BY lead_id ASC LIMIT 1;`, listID).Row().Scan(&leadID)

		// 	if err != nil && err != sql.ErrNoRows {
		// 		return fmt.Errorf("cannot query from leads. %v", err)
		// 	}

		// 	if err == sql.ErrNoRows {
		// 		return errors.New("no dialable leads")
		// 	}

		// }

		// 6. update vicidial_list record
		gor := tx.Exec(`
			UPDATE vicidial_list SET
				status = 'INCALL', user = ?, called_count = (called_count + 1),
				called_since_last_reset = CONCAT('Y', '', called_count)
			WHERE lead_id = ? AND list_id = ?;`,
			params.Username, leadID, listID,
		)

		if gor.Error != nil || gor.RowsAffected < 1 {
			return fmt.Errorf("cannot update lead record. %v", err)
		}

		// 7. grab phone number, campaign cid and conference extension
		var owner, campCID interface{}

		// 8. grab phone number from vicidial list
		// err = tx.Raw(`
		// 	SELECT phone_number,owner FROM vicidial_list
		// 	WHERE user = ? AND list_id = ? AND status = 'INCALL' LIMIT 1;`,
		// 	params.Username, listID).Row().Scan(&phoneNumber, &owner)

		err = tx.Raw(`SELECT phone_number,owner FROM vicidial_list WHERE lead_id = ?`, leadID).Row().Scan(&phoneNumber, &owner)

		if err != nil {
			return fmt.Errorf("cannot find lead_id. %v", err)
		}

		// 9. grab campaign cid from vicidial campaigns
		err = tx.Raw(`SELECT campaign_cid FROM vicidial_campaigns WHERE campaign_id = ?;`, params.Campaign).Row().Scan(&campCID)

		if err != nil {
			return fmt.Errorf("campaign cid for %v does not exist. %v", params.Campaign, err)
		}

		// 10. place external call
		MqueryCID = fmt.Sprintf("M%v%09d", timestamp, leadID)

		pNo := bytes.NewBuffer(phoneNumber.([]byte)).String()

		if err = makeACall(tx, leadID, campCID, params.Username, params.Campaign, MqueryCID, pNo, phone.ServerIP, phone.Extension, "NXDIAL"); err != nil {
			return err
		}

		// if err != nil {
		// 	return fmt.Errorf("cannot find lead_id. %v", err)
		// }

		// 11. update to 'INACTIVE' if it's a callback
		err = tx.Raw(`
			UPDATE vicidial_callbacks
			SET
				status = 'INACTIVE',
				modify_date = NOW()
			WHERE
				phone_number = ? AND
				status IN ('LIVE', 'ACTIVE');`, pNo,
		).Error

		if err != nil {
			return fmt.Errorf("cannot update callback. %v", err)
		}

		// 12. insert user call log
		err = tx.Exec(`
			INSERT INTO user_call_log
				(user, call_date, call_type, server_ip, phone_number, number_dialed, lead_id, campaign_id)
			VALUES
				(?, NOW(), 'MANUAL_DIALNOW', ?, ?, ?, ?, ?);
		`,
			params.Username, phone.ServerIP, pNo, "9"+pNo, leadID, params.Campaign,
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert a new call log. %v", err)
		}

		// 13. prepare url for iframe
		link, err = GetScriptURL(tx, "OUT", fmt.Sprintf("%s", owner), params.Campaign)

		if err != nil {
			return fmt.Errorf("cannot select url for iframe. %v", err)
		}

		return nil
	}()

	// 15. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[DIAL NEXT] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 16. success
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":       http.StatusText(http.StatusOK),
		"callerid":     MqueryCID,
		"lead_id":      fmt.Sprintf("%v", leadID),
		"url":          link,
		"phone_number": bytes.NewBuffer(phoneNumber.([]byte)).String(),
		"unique_id":    uniqueIDVar,
	})
	c.Abort()
	return
}

// ManualDial -
func ManualDial(c *gin.Context) {

	var (
		err             error
		tx              = call.DB.Begin()
		params          = new(manualdial)
		campCID, listID interface{}
		MqueryCID, link string

		leadID, callCount, owner interface{}
	)

	// 1. parse request
	if err = c.Bind(params); err != nil {
		call.Logger.Errorf("cannot parse manual dial request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("manual dial : %+v\n", params)

	// transaction -
	err = func() error {

		// a. fetch phone about user
		var phone = new(models.Phone)

		err = tx.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// b. grab manual list id and campaign cid
		err = tx.Raw(`SELECT campaign_cid, manual_dial_list_id FROM vicidial_campaigns WHERE campaign_id = ?;`, params.Campaign).Row().Scan(&campCID, &listID)

		if err != nil && listID == nil {
			return fmt.Errorf("cannot find manual dial list id for campaign. %v", err)
		}

		if err != nil && campCID == nil {
			return fmt.Errorf("cannot find campaign cid for campaign. %v", err)
		}

		// c. check in leads & hopper. update if exists
		err = tx.Raw(`
			SELECT lead_id, called_count, owner FROM vicidial_list WHERE phone_number = ?;`,
			params.PhoneNumber,
		).Row().Scan(&leadID, &callCount, &owner)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot query from leads. %v", err)
		}

		if leadID != nil {

			// d. delete from hopper
			err = tx.Exec("DELETE FROM vicidial_hopper WHERE lead_id = ?;", leadID).Error

			if err != nil {
				call.Logger.Errorf("[DIAL] cannot delete from hopper. %v", err)
			}

			// e. update vicidial_list record
			err = tx.Exec(`
				UPDATE vicidial_list
				SET
					status = 'INCALL', user = ?, called_count = (called_count + 1),
					called_since_last_reset = CONCAT('Y', '', called_count)
				WHERE lead_id = ?`, params.Username, leadID,
			).Error

			if err != nil {
				call.Logger.Errorf("[DIAL] cannot update vicidial_list. %v", err)
			}

		} else {

			// f. insert into vicidial_list if it doesn't already exisit
			err = tx.Exec(`INSERT INTO vicidial_list
				SET
					phone_code = ?, phone_number = ?,list_id = ?,
					status = ?,  user = ?,  called_since_last_reset = 'Y',
					entry_date = ?,  last_local_call_time = ?, vendor_lead_code = ?;
			`, "1", params.PhoneNumber, listID, "INCALL", params.Username, time.Now(), time.Now(), params.Phone).Error

			if err != nil {
				return fmt.Errorf("cannot insert lead. %v", err)
			}

			// g. now fetch the lead id
			err = tx.Raw(`
				SELECT lead_id FROM vicidial_list WHERE user = ? AND list_id = ? AND status = 'INCALL';`,
				params.Username, listID,
			).Row().Scan(&leadID)

			if err != nil {
				return fmt.Errorf("cannot find lead_id. %v", err)
			}
		}

		// h. make external call
		MqueryCID = fmt.Sprintf("M%v%09d", time.Now().Format("0102150405"), leadID)

		// i. makeACall -
		if err = makeACall(tx, leadID, campCID, params.Username, params.Campaign, MqueryCID, params.PhoneNumber, phone.ServerIP, phone.Extension, "BREAK"); err != nil {
			return err
		}

		// j. insert user call log
		err = tx.Exec(`
			INSERT INTO user_call_log
				(user, call_date, call_type, server_ip, phone_number, number_dialed, lead_id, campaign_id)
			VALUES
				(?, NOW(), 'MANUAL_DIALNOW', ?, ?, ?, ?, ?);
		`,
			params.Username, phone.ServerIP, params.PhoneNumber, "9"+params.PhoneNumber, leadID, params.Campaign,
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert a new call log. %v", err)
		}
		call.Logger.Errorf("INSERTING INTO CALL LOG")

		// var ListID interface{}
		// var userGroup interface{}
		// var callCount interface{}

		// //get the listID
		// err = tx.Raw(`SELECT list_id  FROM vicidial_list where lead_id= ?;`, leadID).Row().Scan(&ListID)
		// err = tx.Raw(`SELECT user_group FROM vicidial_users where user= ?;`, params.Username).Row().Scan(&userGroup)
		// err = tx.Raw(`SELECT called_count FROM vicidial_list WHERE lead_id = ?;`, leadID).Row().Scan(&callCount)

		// // 9. insert into dial log
		// // "INSERT INTO vicidial_log
		// // (uniqueid,lead_id,list_id,campaign_id,call_date,start_epoch,status,phone_code,phone_number,user,comments,processed,user_group,alt_dial,called_count)
		// // values('$uniqueid','$lead_id','$list_id','$campaign','$NOW_TIME','$StarTtime','INCALL','$phone_code','$phone_number','$user','MANUAL','N','$user_group','$alt_dial','$called_count');";

		// err = tx.Exec(`INSERT INTO vicidial_log
		// (    uniqueid,lead_id,list_id,campaign_id, call_date,start_epoch,  status,phone_code,phone_number,user,comments,processed,user_group,alt_dial,called_count) VALUES
		// (            ?,     ?,      ?,          ?,         ?,          ?,'INCALL',        1,                   'MANUAL',      'N',         ?,'MANUAL',    )
		// `, uniqueID(), leadID, ListID, params.Campaign, time.Now(), epochTime(), params.PhoneNumber, params.Username, userGroup, callCount).Error

		// h. update to 'INACTIVE' if it's a callback
		err = tx.Raw(`
			UPDATE vicidial_callbacks
			SET
				status = 'INACTIVE',
				modify_date = NOW()
			WHERE
				phone_number = ? AND
				status IN ('LIVE', 'ACTIVE');`, params.PhoneNumber,
		).Error

		if err != nil {
			return fmt.Errorf("cannot update callback. %v", err)
		}

		// i. prepare url for iframe
		link, err = GetScriptURL(tx, "OUT", fmt.Sprintf("%s", owner), params.Campaign)

		if err != nil {
			return fmt.Errorf("cannot select url for iframe. %v", err)
		}

		return nil
	}()

	// 17. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[MANUAL DIAL] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 18. success
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":    http.StatusText(http.StatusOK),
		"callerid":  MqueryCID,
		"lead_id":   fmt.Sprintf("%v", leadID),
		"unique_id": uniqueIDVar,
		"url":       link,
	})
	c.Abort()
	return
}

// makeACall -
func makeACall(tx *gorm.DB, leadID, cid interface{}, uname, camp, MqueryCID, phoneNumber, serverIP, ext, subStatus string) error {

	// 1. fetch conference extension
	conf, err := GetConference(tx, serverIP, "SIP/"+ext)

	uniqueID()

	var prefix interface{}

	err = tx.Raw(`SELECT dial_prefix FROM vicidial_campaigns WHERE campaign_id = ?;`, camp).Row().Scan(&prefix)

	if err != nil {
		return fmt.Errorf("cannot find campaign. %v", err)
	}

	if err != nil {
		return err
	}

	// 2. place call
	data := models.VicidialManager{
		EntryDate: time.Now(),
		Status:    manager.NEW,
		Response:  manager.NO,
		ServerIP:  serverIP,
		Action:    "Originate",
		CallerID:  MqueryCID,
		CmdLineB:  fmt.Sprintf("Exten: %s%s", prefix, phoneNumber), //fmt.Sprintf("Exten: 1%v", phoneNumber),
		CmdLineC:  "Context: default",
		CmdLineD:  fmt.Sprintf("Channel: Local/%d@default", conf),
		CmdLineE:  "Priority: 1",
		CmdLineF:  fmt.Sprintf("Callerid: \"%v\" <%s>", MqueryCID, cid),
		CmdLineG:  "Timeout: 60000",
	}

	if err = data.Save(tx); err != nil {
		return fmt.Errorf("cannot initiate manual dial. %v", err)
	}

	// 3. record call
	var (
		exten   interface{}
		channel = fmt.Sprintf("Local/5%d@default", conf)

		fullDate = time.Now().Format("20060102-150405")
		filename = fmt.Sprintf("%v_%v", fullDate, phoneNumber)
	)

	err = tx.Raw(`SELECT campaign_rec_exten FROM vicidial_campaigns WHERE campaign_id = ?;`, camp).Row().Scan(&exten)

	if err != nil {
		return fmt.Errorf("cannot find campaign recording extension. %v", err)
	}

	record := &models.VicidialManager{
		EntryDate: time.Now(),
		Status:    manager.NEW,
		Response:  manager.NO,
		ServerIP:  serverIP,
		Action:    "Originate",
		CallerID:  filename[:17] + "...",
		CmdLineB:  fmt.Sprintf("Channel: %s", channel),
		CmdLineC:  "Context: default",
		CmdLineD:  fmt.Sprintf("Exten: %s", exten),
		CmdLineE:  "Priority: 1",
		CmdLineF:  "Callerid: " + filename,
	}

	if err = record.Save(tx); err != nil {
		return fmt.Errorf("cannot record ongoing call. %v", err)

	}

	// 4. insert into recording logs
	var recordingID interface{}

	err = tx.Exec(`
		INSERT INTO recording_log
			(channel,server_ip,extension,start_time,start_epoch,filename,lead_id,user,vicidial_id)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?);
	`, channel, serverIP, exten, time.Now(), time.Now().Unix(),
		filename, leadID, uname, MqueryCID,
	).Error

	if err != nil {
		return fmt.Errorf("cannot save to recording log. %v", err)
	}

	// 5. select from recording log
	err = tx.Raw(`SELECT recording_id FROM recording_log WHERE vicidial_id = ?;`, MqueryCID).Row().Scan(&recordingID)

	if err != nil {
		return fmt.Errorf("cannot select recording id. %v", err)

	}

	// 6. insert into initiated recordings
	err = tx.Exec(`
		INSERT INTO routing_initiated_recordings
			(recording_id, filename, launch_time, lead_id, vicidial_id, user, processed)
		VALUES
			(?, ?, ?, ?, ?, ?, ?);
	`, recordingID, filename, time.Now(), leadID, MqueryCID, uname, '0',
	).Error

	if err != nil {
		return fmt.Errorf("cannot insert into routing_initiated_recordings. %v", err)
	}

	// 7. save to vicidial_auto_calls
	err = tx.Exec(`
			INSERT INTO vicidial_auto_calls
				(server_ip,campaign_id,status,lead_id,callerid,phone_code,phone_number,call_time,call_type)
			VALUES
				(?,?,?,?,?,?,?,?,?)`, serverIP, camp, "XFER", leadID, MqueryCID, "1",
		phoneNumber, time.Now(), "OUT").Error

	if err != nil {
		return fmt.Errorf("cannot insert into auto calls table. %v", err)
	}

	// 8. insert into vicidial dial log
	err = tx.Exec(`
			INSERT INTO vicidial_dial_log
				SET
					caller_code = ?, lead_id = ?, server_ip = ?, call_date = ?,
					extension = ?, channel = ?, context = ?, timeout = ?,
					outbound_cid = ?
			`, MqueryCID, leadID, serverIP, time.Now(),
		conf, fmt.Sprintf("Local/%v@default", conf), "default", "60000",
		fmt.Sprintf("Callerid: \"%v\" <%s>", MqueryCID, cid),
	).Error

	if err != nil {
		call.Logger.Errorf("cannot insert into dial log. %v", err)
	}

	var ListID, userGroup, callCount interface{}

	//get the listID
	err = tx.Raw(`SELECT list_id,called_count  FROM vicidial_list where lead_id= ?;`, leadID).Row().Scan(&ListID, &callCount)
	err = tx.Raw(`SELECT user_group FROM vicidial_users where user= ?;`, uname).Row().Scan(&userGroup)

	// 9. insert into dial log

	err = tx.Exec(`
		INSERT INTO vicidial_log(
			uniqueid,
			lead_id,
			list_id,
			campaign_id,
			call_date,
			start_epoch,
			status,
			phone_code,
			phone_number,
			user,
			comments,
			processed,
			user_group,
			alt_dial,
			called_count
			)
			VALUES(
			?,
			?,
			?,
			?,
			?,
			?,
			'INCALL',
			1,
			?,
			?,
			'MANUAL',
			'N',
			?,
			'MANUAL',
			?)`,
		//uniqueID(),
		uniqueIDVar,
		leadID,
		ListID,
		camp,
		time.Now(),
		epochTime(),
		phoneNumber,
		uname,
		userGroup,
		callCount).Error

	// 10. update agent log
	err = tx.Exec(`
		UPDATE vicidial_agent_log
		SET
			wait_epoch = ?,
			lead_id = ?,
			comments = 'MANUAL',
			sub_status = ?
		WHERE agent_log_id = ? AND lead_id IS NULL ORDER BY event_time DESC LIMIT 1;
		`, time.Now().Unix(), fmt.Sprintf("%v", leadID), subStatus, GetAgentLogID(tx, uname),
	).Error

	if err != nil {
		return err

	}

	// 11. update live agent
	err = tx.Exec(`
		UPDATE vicidial_live_agents
			SET
				status = 'INCALL',
				uniqueid = '0',
				last_call_time = NOW(),
				callerid = ?,
				channel = '',
				lead_id = ?,
				comments = 'MANUAL',
				calls_today = (calls_today + 1),
				external_hangup = 0,
				external_status = '',
				external_pause = '',
				external_dial = '',
				external_recording = ?,
				last_state_change = NOW(),
				pause_code = ''
			WHERE user = ? AND server_ip = ?;
	`, MqueryCID, leadID, recordingID, uname, serverIP,
	).Error

	if err != nil {
		return fmt.Errorf("cannot update live agent data. %v", err)
	}

	// 12. insert into recent session
	err = tx.Exec(`
		INSERT INTO vicidial_sessions_recent
		SET
			lead_id=?,
			server_ip=?,
			call_date=NOW(),
			user=?,
			campaign_id=?,
			conf_exten=?,
			call_type='M';`,
		leadID, serverIP, uname,
		camp, exten,
	).Error

	if err != nil {
		return err
	}

	return nil
}
