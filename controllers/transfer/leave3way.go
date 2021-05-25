package transfer

import (
	"fmt"
	"net/http"
	"smartdial/controllers/calls"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"time"

	"github.com/gin-gonic/gin"
)

// Leave3Way - leave others (hang all chans)
func Leave3Way(c *gin.Context) {

	var (
		err    error
		params = new(leave3way)
		phone  = new(models.Phone)
		tx     = call.DB.Begin()
	)

	// 1. parse request
	if err = c.Bind(params); err != nil {
		call.Logger.Errorf("cannot parse transfer parked request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("leave3way : %+v \n", params)

	// 2. begin  transaction
	err = func() error {

		var conf, channel interface{}

		// get phone detail
		err = call.DB.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// fetch conference extension
		conf, err = calls.GetConference(tx, phone.ServerIP, "SIP/"+phone.Extension)

		if err != nil {
			return err
		}

		// update conference
		err = tx.Exec(`
			UPDATE vicidial_conferences 
			SET 
				extension = ?,
				leave_3way = '1',
				leave_3way_datetime = NOW()
			WHERE 
				conf_exten = ? 
		`, "3WAY_"+params.Username, conf).Error

		if err != nil {
			return fmt.Errorf("cannot update conferences to leave_3way. %v", err)
		}

		// fetch live channel
		err := tx.Raw(`
			SELECT channel FROM live_sip_channels 
			WHERE 
				server_ip = ? AND 
				extension = ? AND  
				channel_data = ?;`,
			phone.ServerIP, conf, fmt.Sprintf(`%d,Fmq`, conf),
		).Row().Scan(&channel)

		if err != nil {
			return fmt.Errorf("cannot select live channels. %v", err)
		}

		// stop monitor your channel
		hangup := &models.VicidialManager{
			EntryDate: time.Now(),
			Status:    manager.NEW,
			Response:  manager.NO,
			ServerIP:  phone.ServerIP,
			Action:    "Hangup",
			CallerID:  fmt.Sprintf("RH12345%v0", time.Now().Unix()),
			CmdLineB:  fmt.Sprintf("Channel: %s", channel),
		}

		if err = hangup.Save(tx); err != nil {
			return fmt.Errorf("cannot up live channels. %v", err)
		}

		// fetch agent channel
		err = tx.Raw(`
			SELECT channel FROM live_sip_channels 
			WHERE 
				server_ip = ? AND 
				extension = ? AND  
				channel LIKE ?;`,
			phone.ServerIP, conf, fmt.Sprintf(`SIP/%s`, phone.Extension)+"%",
		).Row().Scan(&channel)

		if err != nil {
			return fmt.Errorf("cannot select live channels. %v", err)
		}

		// switch to another conference
		conf, err = calls.GetConference(tx, phone.ServerIP, "SIP/"+phone.Extension)

		if err != nil {
			return err
		}

		// redirect xtra new
		callerID := fmt.Sprintf("CXAR23%v", time.Now().Format("20060102150405"))

		xtra := &models.VicidialManager{
			EntryDate: time.Now(),
			Status:    manager.NEW,
			Response:  manager.NO,
			ServerIP:  phone.ServerIP,
			Action:    "Redirect",
			CallerID:  callerID,
			CmdLineB:  fmt.Sprintf("Channel: %s", channel),
			CmdLineC:  fmt.Sprintf("Context: default"),
			CmdLineD:  fmt.Sprintf("Exten: %d", conf),
			CmdLineE:  "Priority: 1",
			CmdLineF:  fmt.Sprintf("CallerID: %v", callerID),
		}

		if err = xtra.Save(tx); err != nil {
			return fmt.Errorf("cannot up live channels. %v", err)
		}

		return nil
	}()

	// 3. error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[3WAY - LEAVE] %v", err)
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
