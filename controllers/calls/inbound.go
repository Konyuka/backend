package calls

import (
	"fmt"
	"net/http"
	"smartdial/data"
	"smartdial/log"
	"smartdial/models"
	"smartdial/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

	// register closer inbound groups
	regcloser struct {
		model
		Groups  []string `json:"groups"`
		Blended string   `json:"blended"`
	}

	// hanging up a call
	hangup struct {
		model
		CallerID    string `json:"callerid"`
		LeadID      string `json:"lead_id"`
		PhoneNumber string `json:"phone_number"`
	}

	// dispose hanged up call
	dispose struct {
		model
		Status    string `json:"status"`
		LeadID    string `json:"lead_id"`
		PauseCode string `json:"pause_code"`

		// callback stuff
		Recipient    string `json:"recipient"`
		CallbackTime string `json:"callback_time"`
		Comment      string `json:"comments"`
		Type         string `json:"call_type"`
		UniqueID     string `json:"unique_id"`
	}

	// manual dial
	manualdial struct {
		model
		PhoneNumber string `json:"phone_number"`
		CallbackID  string `json:"callback_id"`
	}

	// dialnext - lead from list
	dialnext struct {
		model
	}

	// park/hold ongoing call
	parkcall struct {
		model
		CallerID string `json:"callerid"`
		LeadID   string `json:"lead_id"`
	}

	// grabcall - from hold
	grabcall struct {
		model
		CallerID string `json:"callerid"`
	}

	// takecall - on queue
	takecall struct {
		model
		AutoCallID  string `json:"call_id"`
		PhoneNumber string `json:"phone_number"`
		LeadID      string `json:"lead_id"`
	}

	// switch pause code
	changepause struct {
		model
		PauseCode string `json:"pause_code"`
	}

	// toggle pause/ready
	togglepause struct {
		model
		State     string `json:"state"`
		PauseCode string `json:"pause_code"`
	}

	// dtmf
	dtmf struct {
		model
		CallerID string `json:"callerid"`
	}

	// callogs
	callogs struct {
		model
		Date string `json:"date"`
	}
)

var call = &Action{
	DB:     data.GetDB(),
	Logger: log.GetLogger(),
}

// InboundGroup - check in inbound groups
func InboundGroup(c *gin.Context) {

	var (
		err    error
		param  = new(regcloser)
		phone  = new(models.Phone)
		groups = make(map[string]bool)
		tx     = call.DB.Begin()
	)

	// 1. parse request
	if err = c.BindJSON(param); err != nil {
		call.Logger.Errorf("could not parse request for inbound groups registration : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("inbound groups : %+v\n", param)

	// 2. update closer campaigns
	err = func() error {

		// fetch allowed inbound groups
		allowed, err := AllowedInboundGroups(tx, param.Campaign)

		if err != nil {
			return err
		}

		var cc string // closer campaigns

		if utils.Find(allowed, "AGENTDIRECT") {
			cc = strings.Join(param.Groups, " ") + " AGENTDIRECT -"
		}

		param.Groups = append(param.Groups, "AGENTDIRECT")

		// grap phone details
		if err = tx.Raw(`SELECT * FROM phones WHERE extension = ?`, param.Phone).Scan(phone).Error; err != nil {
			return err
		}

		if len(param.Blended) < 1 {
			param.Blended = "0"
		}

		var autoDial string

		if param.Blended == "0" {
			autoDial = "N"
		} else if param.Blended == "1" {
			autoDial = "Y"
		}

		// update live agents
		gor := tx.Exec(`UPDATE vicidial_live_agents
			SET
				status = IF(outbound_autodial = 'Y', 'READY', status),
				closer_campaigns = ?,
				last_state_change = ?,
				outbound_autodial = ?,
				external_blended = ?
			WHERE
				user = ? AND
				server_ip = ?;

		`, cc, time.Now(), autoDial, param.Blended, param.Username, phone.ServerIP)

		if gor.RowsAffected < 1 || gor.Error != nil {
			return fmt.Errorf("live agent not updated. %v", gor.Error)
		}

		// add user closer log
		err = tx.Exec(`
			INSERT INTO vicidial_user_closer_log
			SET
				user = ?,
				campaign_id = ?,
				event_date = NOW(),
				blended = ?,
				closer_campaigns = ?;
		`, param.Username, param.Campaign, param.Blended, cc,
		).Error

		if err != nil {
			return err
		}

		// update vicidial users
		err = tx.Exec(`UPDATE vicidial_users SET closer_campaigns = ? WHERE user = ?;`, cc, param.Username).Error

		if err != nil {
			return err
		}

		// delete live inbound agents
		err = tx.Exec(`DELETE FROM vicidial_live_inbound_agents WHERE user = ?;`, param.Username).Error

		if err != nil {
			return fmt.Errorf("cannot delete records from live inbound. %v", err)
		}

		// fetch inbound groups for this user
		groups, err = MyGroups(tx, param.Campaign, param.Groups)

		if err != nil {
			return err
		}

		// save inbound groups
		if err = AddInbound(tx, param.Username, param.Groups); err != nil {
			return fmt.Errorf("cannot delete from inbound live agents %v", err)
		}

		return nil
	}()

	// 3. error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[INGROUPS] %v", err)
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
		"status":  http.StatusText(http.StatusOK),
		"inbound": groups,
	})
	c.Abort()
	return
}
