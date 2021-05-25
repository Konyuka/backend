package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"smartdial/controllers/calls"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LoginToCampaign - once the user is authorized we can login phone to asterisk
func LoginToCampaign(c *gin.Context) {

	var (
		param = new(plogin)
		err   error
		tx    = auth.DB.Begin()

		phone, userGroup        interface{}
		pauseAfter              interface{}
		agentPauseAfterEachCall interface{}
		serverIP                interface{}
		dialMethod              interface{}
		dialableLeads           interface{}

		groups     map[string]bool
		dispos     = []map[string]string{}
		pauseCodes = []map[string]string{}

		pauseEpoch = time.Now().Unix()

		pauseCode = "LOGIN"

		agentLogID int64
	)

	// 1. parse login to campaign request
	if err = c.Bind(param); err != nil {
		auth.Logger.Errorf("cannot extract campaign id from body : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("error reading phone login from request"),
		})
		c.Abort()
		return
	}

	fmt.Printf("login campaign : %+v\n", param)

	// login to phone transaction
	err = func() error {

		//  2. pick up phone_login
		err = tx.Raw(`
			SELECT phone_login, user_group FROM vicidial_users WHERE  user = ?;`, param.Username).Row().Scan(&phone, &userGroup)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if phone == nil || err == sql.ErrNoRows {
			return errors.New("selected user doesn't have a phone login")
		}

		// 3. check if user already logged in
		sipExt := fmt.Sprintf("SIP/%s", phone) + "%"
		var channel interface{}
		err = tx.Raw(`SELECT channel FROM live_sip_channels WHERE channel LIKE ?;`, sipExt).Row().Scan(&channel)

		// there's an error and it's not about absence of channel
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if channel != nil {

			// logout phone
			if err = LogoutPhone(tx, fmt.Sprintf("%s", phone)); err != nil {
				return err
			}

			time.Sleep(2 * time.Second)
		}

		// 5. select groups
		groups, err = calls.MyGroups(tx, param.Campaign, []string{})

		if err != nil {
			return err
		}

		// 6. add AGENTDIRECT to inbound group
		if err = calls.AddInbound(tx, param.Username, []string{"AGENTDIRECT"}); err != nil {
			return err
		}

		// 7. prepare dispositions
		rows, err := tx.Raw(`
			SELECT status, status_name FROM vicidial_statuses WHERE selectable = 'Y' AND human_answered ='Y';
		`).Rows()

		if err != nil {
			return err
		}

		for rows.Next() {

			var status, name interface{}

			if err = rows.Scan(&status, &name); err != nil {
				return err
			}

			dispos = append(dispos, map[string]string{
				"name":  fmt.Sprintf("%s", name),
				"value": fmt.Sprintf("%s", status),
			})
		}

		// 8. pause after each call
		err = tx.Raw(`SELECT pause_after_each_call,agent_pause_codes_active FROM vicidial_campaigns WHERE campaign_id = ?;`, param.Campaign).Row().Scan(&pauseAfter, &agentPauseAfterEachCall)

		if err != nil {
			return err
		}

		// 9. get pause codes
		pauseCodes, err = getPauseCodes(param.Campaign)

		if err != nil {
			return fmt.Errorf("cannot get pause codes. %v", err)
		}

		// 10. get server ip
		if err = tx.Raw(`SELECT server_ip FROM phones WHERE extension = ?`, phone).Row().Scan(&serverIP); err != nil {
			return fmt.Errorf("cannot find server ip.%v", err)
		}

		// 11. get dial method
		err = tx.Raw(`SELECT dial_method FROM vicidial_campaigns WHERE campaign_id = ?;`, param.Campaign).Row().Scan(&dialMethod)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot select dial_method for campaign %v. %v", param.Campaign, err)
		}

		if dialMethod == nil || err == sql.ErrNoRows {
			return fmt.Errorf("selected campaign does not exist")
		}

		// 12. insert agent log
		query := fmt.Sprintf(`
			INSERT INTO vicidial_agent_log
				(user, server_ip, event_time, campaign_id, pause_epoch, wait_sec, wait_epoch, user_group, sub_status, pause_type)
			VALUES
				('%v', '%v', '%v', '%v', %v, %v, %v, '%v', '%v', '%v')
			ON DUPLICATE KEY UPDATE agent_log_id = LAST_INSERT_ID(agent_log_id);
		`, param.Username, fmt.Sprintf("%s", serverIP), time.Now().Format("2006-01-02 15:04:00"),
			strings.ToUpper(param.Campaign), pauseEpoch, 0, pauseEpoch, fmt.Sprintf("%s", userGroup), pauseCode, "AGENT",
		)

		// prepare query
		stmt, err := tx.CommonDB().Prepare(query)

		if err != nil {
			return err
		}

		// execute query
		res, err := stmt.Exec()

		if err != nil {
			return err
		}

		//get the number of the leads
		err = tx.Raw(`SELECT COUNT(*) FROM vicidial_list vl INNER JOIN vicidial_lists vls ON vl.list_id = vls.list_id WHERE vls.campaign_id = ? AND vl.status = 'NEW';`, param.Campaign).Row().Scan(&dialableLeads)

		// grab the inserted id
		agentLogID, err = res.LastInsertId()

		fmt.Printf("Agentlog id : %v\n", agentLogID)

		if err != nil {
			return err
		}

		param.AgentLogID = agentLogID

		// log phone to a campaign
		if err = loginPhone(tx, param); err != nil {
			return err
		}

		return nil
	}()

	// 14. error response and rollback
	if err != nil {
		tx.Rollback()
		auth.Logger.Errorf("cannot login to campaign : %v", err)
		c.JSON(http.StatusForbidden, gin.H{
			"status": http.StatusText(http.StatusForbidden),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 15. check if user picked up
	if err = loginAgentWithTimeout(fmt.Sprintf("%s", phone), 27); err != nil {
		tx.Rollback()
		auth.Logger.Infof("%v", err)
		c.JSON(http.StatusExpectationFailed, gin.H{
			"status": http.StatusText(http.StatusExpectationFailed),
			"error":  fmt.Sprintf("%s", err),
		})
		c.Abort()
		return
	}

	// 15. success response
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":                      http.StatusText(http.StatusOK),
		"pause_code":                  pauseCode,
		"phone":                       fmt.Sprintf("%s", phone),
		"inbound":                     groups,
		"dispos":                      dispos,
		"pause_after":                 fmt.Sprintf("%s", pauseAfter),
		"pause_codes":                 pauseCodes,
		"dial_method":                 fmt.Sprintf("%s", dialMethod),
		"agent_log_id":                agentLogID,
		"agent_pause_after_each_call": fmt.Sprintf("%s", agentPauseAfterEachCall),
		"leads":                       dialableLeads,
	})
	c.Abort()
	return
}
