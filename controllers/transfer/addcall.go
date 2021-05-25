package transfer

import (
	"fmt"
	"net/http"
	"smartdial/controllers/calls"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// AddCall - add call
func AddCall(c *gin.Context) {

	var (
		err    error
		params = new(transferAdd)
		tx     = call.DB.Begin()
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

	fmt.Printf("add call : %+v \n", params)

	// 2. begin transaction
	err = func() error {

		var (
			campCID interface{}
		)

		// a. fetch phone about user
		var phone = new(models.Phone)

		err = tx.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// b. grab manual list id and campaign cid
		err = tx.Raw(`SELECT campaign_cid FROM vicidial_campaigns WHERE campaign_id = ?;`, params.Campaign).Row().Scan(&campCID)

		if err != nil && campCID == nil {
			return fmt.Errorf("cannot find campaign cid for campaign. %v", err)
		}

		// fetch conference extension
		conf, err := calls.GetConference(tx, phone.ServerIP, "SIP/"+phone.Extension)

		if err != nil {
			return err
		}

		timestamp := time.Now().Format("0102150405")

		MqueryCID := fmt.Sprintf("DC%v%09vW", timestamp[:6], params.LeadID)

		if _, err := strconv.Atoi(params.Agent); err == nil {
			params.Agent = fmt.Sprintf("1%v", params.Agent)
		}

		// add call -
		data := models.VicidialManager{
			EntryDate: time.Now(),
			Status:    manager.NEW,
			Response:  manager.NO,
			ServerIP:  phone.ServerIP,
			Action:    "Originate",
			CallerID:  MqueryCID,
			CmdLineB:  fmt.Sprintf("Channel: Local/%v@default", params.Agent),
			CmdLineC:  "Context: default",
			CmdLineD:  fmt.Sprintf("Exten: %d", conf),
			CmdLineE:  "Priority: 1",
			CmdLineF:  fmt.Sprintf("Callerid: \"%v\" <%s>", MqueryCID, campCID),
			CmdLineG: fmt.Sprintf(`
				Variable: __vendor_lead_code=%v,__lead_id=%v
			`, params.Phone, params.LeadID),
		}

		if err = data.Save(tx); err != nil {
			return fmt.Errorf("cannot add transfer call. %v", err)
		}

		// save to vicidial_auto_calls
		err = tx.Exec(`
			INSERT INTO vicidial_auto_calls 
				(server_ip,campaign_id,status,lead_id,callerid,phone_code,phone_number,call_time,call_type)
			VALUES 
				(?,?,?,?,?,?,?,?,?)`, phone.ServerIP, params.Campaign, "XFER", params.LeadID, MqueryCID, "1",
			params.Agent, time.Now(), "OUT").Error

		if err != nil {
			return fmt.Errorf("cannot insert into auto calls table. %v", err)
		}

		// insert user call log
		err = tx.Exec(`
			INSERT INTO user_call_log 
				(user, call_date, call_type, server_ip, phone_number, number_dialed, lead_id, campaign_id) 
			VALUES
				(?, NOW(), 'XFER_3WAY', ?, ?, ?, ?, ?)
		`,
			params.Username, phone.ServerIP, params.Agent, "9"+params.Agent, params.LeadID, params.Campaign,
		).Error

		if err != nil {
			return fmt.Errorf("cannot insert a new call log. %v", err)
		}

		// update transfer stat
		gor := tx.Exec(`
			UPDATE vicidial_xfer_stats SET 
			xfer_count = (xfer_count+1) WHERE campaign_id=? ;`,
			params.Campaign,
		)

		if gor.Error != nil {
			return fmt.Errorf("cannot update xfer stat. %v", gor.Error)
		}

		if gor.RowsAffected < 1 {

			err = tx.Exec(`
				INSERT INTO vicidial_xfer_stats 
				SET campaign_id = ?, xfer_count = 1;`,
				params.Campaign,
			).Error

			if err != nil {
				return fmt.Errorf("cannot insert into vicidial xfer stats. %v", err)
			}
		}

		// insert dial log
		err = tx.Exec(`
			INSERT INTO vicidial_dial_log
				SET 
					caller_code = ?, lead_id = ?, server_ip = ?, call_date = ?,
					extension = ?, channel = ?, context = ?, timeout = ?, 
					outbound_cid = ?
			`, MqueryCID, params.LeadID, phone.ServerIP, time.Now(),
			conf, fmt.Sprintf("Local/%v@default", conf), "default", "0",
			fmt.Sprintf("Callerid: \"%v\" <%s>", MqueryCID, data.CmdLineF),
		).Error

		if err != nil {
			call.Logger.Errorf("cannot insert into dial log. %v", err)
		}

		return nil
	}()

	// 3. error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[3WAY - ADDCALL] %v", err)
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
