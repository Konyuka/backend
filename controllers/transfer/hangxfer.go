package transfer

import (
	"fmt"
	"net/http"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"time"

	"github.com/gin-gonic/gin"
)

// HangupXFERLine - hangups the recently added transfer line
func HangupXFERLine(c *gin.Context) {

	var (
		err    error
		params = new(hangxfer)
		phone  = new(models.Phone)
		tx     = call.DB.Begin()
	)

	// 1. parse request
	if err = c.Bind(params); err != nil {
		call.Logger.Errorf("cannot parse hang xfer line request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("hangup xfer line : %+v \n", params)

	// 2. begin  transaction
	err = func() error {

		var channel interface{}

		// get phone detail
		err = call.DB.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// fetch live channel
		like := "%" + params.Agent + "%"

		err := tx.Raw(`
			SELECT channel FROM live_sip_channels 
			WHERE 
				server_ip = ? AND 
				extension LIKE ? ;`,
			phone.ServerIP, like,
		).Row().Scan(&channel)

		if err != nil {
			return fmt.Errorf("cannot select live channels. %v", err)
		}

		// stop monitor xfer-line channel
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

		return nil
	}()

	// 3. error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[3WAY - HANGUP XFERLINE] %v", err)
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
