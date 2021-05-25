package calls

import (
	"fmt"
	"net/http"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"time"

	"github.com/gin-gonic/gin"
)

// SendDTMF -
func SendDTMF(c *gin.Context) {

	var (
		params = new(dtmf)
		err    error

		tx = call.DB.Begin()
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse dtmf request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("dtmf payload : %+v\n", params)

	// 2. send the details
	err = func() error {

		// a. get phone details
		var phone = new(models.Phone)

		err = tx.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		// b. get conference
		conf, err := GetConference(tx, phone.ServerIP, "SIP/"+phone.Login)

		if err != nil {
			return err
		}

		// c. manager command
		data := &models.VicidialManager{
			EntryDate: time.Now(),
			Status:    manager.NEW,
			Response:  manager.NO,
			ServerIP:  phone.ServerIP,
			Action:    "Originate",
			CallerID:  params.CallerID,
			CmdLineB:  fmt.Sprintf("Channel: %v", phone.DTMFSendExtension),
			CmdLineC:  fmt.Sprintf("Context: default"),
			CmdLineD:  fmt.Sprintf("Exten: 7%v", conf),
			CmdLineE:  fmt.Sprintf("Priority: 1"),
			CmdLineF:  fmt.Sprintf("Callerid: %v", params.CallerID),
		}

		if err = data.Save(tx); err != nil {
			return fmt.Errorf("cannot send dtmf command. %v", err)
		}

		return nil
	}()

	// 11. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[SEND DTMF] %v", err)
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
	})
	c.Abort()
	return
}
