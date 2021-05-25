package auth

import (
	"database/sql"
	"fmt"
	"net/http"
	"smartdial/controllers/calls"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SwitchCampaign -
func SwitchCampaign(c *gin.Context) {

	var (
		err        error
		param      = new(changecamp)
		tx         = auth.DB.Begin()
		groups     = make(map[string]bool)
		pauseCodes = []map[string]string{}

		dialMethod interface{}
		serverIP   interface{}
		userGroup  interface{}

		pauseEpoch = time.Now().Unix()

		pauseCode = ""
		status    = "READY"
	)

	// 1. parse request data
	if err = c.Bind(param); err != nil {
		auth.Logger.Errorf("cannot parse request : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("error parsing request body. %v", err),
		})
		c.Abort()
		return
	}

	// switch campaign transaction
	err = func() error {

		// 2. check if campaign exists and fetch dial_method
		err = tx.Raw(`SELECT dial_method FROM vicidial_campaigns WHERE campaign_id = ?`, param.Campaign).Row().Scan(&dialMethod)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot select dial_method for campaign %v. %v", param.Campaign, err)
		}

		if dialMethod == nil || err == sql.ErrNoRows {
			return fmt.Errorf("selected campaign does not exist")
		}

		// 3. delete from live agent
		err = tx.Exec(`DELETE FROM vicidial_live_agents WHERE user = ? AND extension LIKE ?;`, param.Username, "%"+param.Phone+"%").Error

		if err != nil {
			return err
		}

		// 4. delete from inbound group
		err = tx.Exec(`DELETE FROM vicidial_live_inbound_agents WHERE user = ?;`, param.Username).Error
		if err != nil {
			return err
		}

		// 5. add AGENTDIRECT to inbound group
		if err = calls.AddInbound(tx, param.Username, []string{"AGENTDIRECT"}); err != nil {
			return err
		}

		groups, err = calls.MyGroups(tx, param.Campaign, []string{})

		if err != nil {
			return err
		}

		// 6. logout phone
		if err = LogoutPhone(tx, param.Phone); err != nil {
			return fmt.Errorf("logout and login again. %v", err)
		}

		time.Sleep(3 * time.Second)

		// 7. login phone to new campaign
		if err = loginPhone(tx, &plogin{Campaign: param.Campaign, Username: param.Username}); err != nil {
			return fmt.Errorf("logout and login again. %v", err)
		}

		// 8. get pause codes
		pauseCodes, err = getPauseCodes(param.Campaign)

		if err != nil {
			return fmt.Errorf("cannot get pause codes. %v", err)
		}

		// 9. get server ip
		if err = tx.Raw(`SELECT server_ip FROM phones WHERE extension = ?`, param.Phone).Row().Scan(&serverIP); err != nil {
			return fmt.Errorf("cannot find server ip.%v", err)
		}

		//  10. pick up userGroup
		err = tx.Raw(`SELECT user_group FROM vicidial_users WHERE  user = ?;`, param.Username).Row().Scan(&userGroup)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// 9. insert agent log
		if dialMethod != fmt.Sprintf("%s", "RATIO") {
			pauseCode = "LOGIN"
			status = "PAUSED"
		}

		err = tx.Exec(`
				INSERT INTO vicidial_agent_log
					(user, server_ip, event_time, campaign_id, pause_epoch, wait_sec, wait_epoch, user_group, sub_status, pause_type)
				VALUES 
					(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, param.Username, fmt.Sprintf("%s", serverIP), time.Now(), strings.ToUpper(param.Campaign),
			pauseEpoch, 0, pauseEpoch, fmt.Sprintf("%s", userGroup), pauseCode, "AGENT").Error

		if err != nil {
			return fmt.Errorf("cannot insert agent log. %v", err)
		}

		// 12. update agent log id for live agent
		err = tx.Exec(`
				UPDATE vicidial_live_agents SET status = ?, agent_log_id = ? AND pause_code = ? WHERE user = ?;`,
			status, calls.GetAgentLogID(tx, param.Username), pauseCode, param.Username,
		).Error

		if err != nil {
			return fmt.Errorf("cannot update live agent. %v", err)
		}

		return nil
	}()

	// 9. handle error and rollback
	if err != nil {
		tx.Rollback()
		auth.Logger.Errorf("cannot switch campaign. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("error switching campaign. %v", err),
		})
		c.Abort()
		return
	}

	// 10. check if user picked up
	if err = loginAgentWithTimeout(param.Phone, 27); err != nil {
		auth.Logger.Infof("%v", err)
		c.JSON(http.StatusExpectationFailed, gin.H{
			"status": http.StatusText(http.StatusExpectationFailed),
			"error":  fmt.Sprintf("%s", err),
		})
		c.Abort()
		return
	}

	// 11. success campaign switch
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":      http.StatusText(http.StatusOK),
		"state":       status,
		"pause_code":  pauseCode,
		"inbound":     groups,
		"pause_codes": pauseCodes,
		"dial_method": fmt.Sprintf("%s", dialMethod),
	})
	c.Abort()
	return
}
