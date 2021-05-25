package calls

import (
	"database/sql"
	"fmt"
	"net/http"
	"smartdial/models"

	//"smartdial/utils"

	"github.com/gin-gonic/gin"
)

// TakeCall - connect to incomming call
func TakeCall(c *gin.Context) {

	var (
		err    error
		params = new(takecall)
		tx     = call.DB.Begin()
		phone  = new(models.Phone)
		link   string
	)

	// 1. parse request
	if err = c.Bind(params); err != nil {
		call.Logger.Errorf("cannot parse take call request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("take call : %+v\n", params)

	// 2. begin take call transaction
	err = func() error {

		// a. update auto calls to user

		//also update the server ip   //,
		//server_ip = ?  //utils.GetLocalIP(),
		err = tx.Exec(`
			UPDATE vicidial_auto_calls
			SET
				agent_grab = ?
			WHERE auto_call_id = ?;
		`, params.Username, params.AutoCallID).Error

		if err != nil {
			return fmt.Errorf("cannot update auto calls. %v", err)
		}

		// b. select call details
		var uniqueID, callTime, quePriority, callType interface{}
		var serverIp string

		err = tx.Raw(`
			SELECT
				uniqueid, call_time, queue_priority, call_type,server_ip
			FROM vicidial_auto_calls
			WHERE agent_grab = ? AND (status = 'LIVE' OR status = 'XFER');
		`, params.Username).Row().Scan(&uniqueID, &callTime, &quePriority, &callType, &serverIp)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot grab call details from auto calls. %v", err)
		}

		// c. save a grabbed call log
		err = tx.Exec(`
			INSERT INTO vicidial_grab_call_log
			SET
				auto_call_id = ?, user = ?, event_date = NOW(), call_time = ?,
				campaign_id = ?, uniqueid = ?, phone_number = ?, lead_id = ?,
				queue_priority = ?, call_type = ?;
		`, params.AutoCallID, params.Username, callTime, params.Campaign,
			uniqueID, params.PhoneNumber, params.LeadID, quePriority, callType,
		).Error

		if err != nil {
			return fmt.Errorf("cannot save a new grab call log. %v", err)
		}

		// d. get phone detail
		err = call.DB.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// e. insert user call log
		err = tx.Exec(`
			INSERT INTO user_call_log
				(user, call_date, call_type, server_ip, phone_number, number_dialed, lead_id, campaign_id)
			VALUES
				(?, NOW(), 'MAIN', ?, ?, ?, ?, ?);
		`,
			params.Username, serverIp, params.PhoneNumber, "9"+params.PhoneNumber, params.LeadID, params.Campaign,
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert a new call log. %v", err)
		}

		// f. select callerid
		var callerid interface{}

		err = tx.Raw(`SELECT callerid FROM vicidial_auto_calls WHERE auto_call_id = ?;`, params.AutoCallID).Row().Scan(&callerid)

		if err != nil {
			return fmt.Errorf("cannot select callerid from vicidial_auto_calls. %v", err)
		}

		// g. Fetch channel
		var channel interface{}

		//serverIP = utils.GetLocalIP()

		err = tx.Raw(`
			SELECT channel FROM vicidial_auto_calls
			WHERE
				lead_id = ? AND
				server_ip = ? AND
				phone_number = ? AND
				channel LIKE ? AND
				(status = 'CLOSER' OR  status = 'LIVE');`,
			params.LeadID, serverIp, params.PhoneNumber, "SIP%",
		).Row().Scan(&channel)

		if err != nil {
			return fmt.Errorf("cannot fetch live channel %v", err)
		}

		// h. update user to live call
		err = tx.Exec(`
			UPDATE vicidial_live_agents
			SET
				status = 'INCALL',
				lead_id = ?,
				callerid = ?,
				channel = ?
			WHERE
				user = ? AND
				campaign_id = ?;`,
			params.LeadID, callerid, channel,
			params.Username, params.Campaign,
		).Error

		// i. prepare url for iframe
		link, err = GetScriptURL(tx, "IN", params.PhoneNumber, params.Campaign)

		if err != nil {
			return fmt.Errorf("cannot select url for iframe. %v", err)
		}

		return err
	}()

	// 3. error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[TAKE CALL (QUEUE)]%v", err)
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
		"url":    link,
	})
	c.Abort()
	return
}
