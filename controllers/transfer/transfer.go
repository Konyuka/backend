package transfer

import (
	"database/sql"
	"fmt"
	"smartdial/data"
	"smartdial/log"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type (

	// Action -
	Action struct {
		DB     *gorm.DB
		Logger *logrus.Logger
	}

	// base model
	model struct {
		Username string `json:"username"`
		Phone    string `json:"phone"`
		Campaign string `json:"campaign"`
	}

	transfer struct {
		model
		CallerID    string `json:"callerid"`
		LeadID      string `json:"lead_id"`
		Group       string `json:"inbound_group"`
		PhoneNumber string `json:"phone_number"`
		Agent       string `json:"agent"`
	}

	transferAdd struct {
		model
		LeadID string `json:"lead_id"`
		Agent  string `json:"agent"`
	}

	dialparked struct {
		model
		CallerID string `json:"callerid"`
		LeadID   string `json:"lead_id"`
		Agent    string `json:"agent"`
	}

	hangxfer struct {
		model
		Agent string `json:"agent"`
	}

	leave3way struct {
		model
	}
)

var call = &Action{
	DB:     data.GetDB(),
	Logger: log.GetLogger(),
}

// redirect -
func redirect(params *transfer, queryID, serverIP, exten string) error {

	var (
		err      error
		channel  interface{}
		queryCID = fmt.Sprintf("%v%v%v", queryID, time.Now().Unix(), params.Username)
	)

	// fetch call channel
	err = call.DB.Raw(`SELECT channel FROM vicidial_manager WHERE callerid = ?;`, params.CallerID).Row().Scan(&channel)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("cannot select call details from manager. %v", err)
	}

	if err != nil || channel == nil {

		err = call.DB.Raw(`SELECT channel FROM vicidial_auto_calls WHERE auto_call_id = ?;`, params.CallerID).Row().Scan(&channel)

		if err != nil {
			return fmt.Errorf("cannot select call details from vicidial_auto_calls. %v", err)
		}
	}

	// send request to manager
	vals := &models.VicidialManager{
		EntryDate: time.Now(),
		Status:    manager.NEW,
		Response:  manager.NO,
		ServerIP:  serverIP,
		Action:    "Redirect",
		CallerID:  queryCID,
		CmdLineB:  fmt.Sprintf("Channel: %s", channel),
		CmdLineC:  "Context: default",
		CmdLineD:  "Exten: " + exten,
		CmdLineE:  "Priority: 1",
		CmdLineF:  "CallerID: " + queryCID,
	}

	if err := vals.Save(call.DB); err != nil {
		return err
	}

	return nil
}
