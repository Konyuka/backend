package auth

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"smartdial/controllers/calls"
	"smartdial/models"
	"smartdial/models/constants/manager"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

// loginPhone - logs a phone to a campaign
func loginPhone(tx *gorm.DB, param *plogin) error {

	var (
		err     error
		campCID interface{}
		conf    interface{}

		phone     = new(models.Phone)
		user      = new(models.User)
		timestamp = time.Now().Format("060102150405")
	)

	// 1. select user & campaign details
	err = tx.Raw(`
			SELECT * FROM vicidial_users 
			WHERE 
				user = ? AND  active = 'Y' AND failed_login_count = 0;`, param.Username,
	).Scan(&user).Error

	if err != nil {
		return err
	}
	// phone
	tx.Find(phone, "extension = ?", user.PhoneLogin)
	// campain
	err = tx.Raw(`SELECT campaign_cid FROM vicidial_campaigns WHERE campaign_id = ?;`, param.Campaign).Row().Scan(&campCID)

	if err != nil {
		return fmt.Errorf("campaign doesn't exist. %v", err)
	}

	// 2. prepare live agent data
	var (
		campW, callT, campGrade interface{}
		nowTime                 = time.Now()
		random                  = rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1000000)
	)

	err = tx.Raw(`
		SELECT campaign_weight,calls_today,campaign_grade FROM vicidial_campaign_agents 
		WHERE user=? AND campaign_id=?;`, param.Username, param.Campaign,
	).Row().Scan(&campW, &callT, &campGrade)

	if campW == nil {
		campW = 0
	}

	if callT == nil {
		callT = 0
	}

	if campGrade == nil {
		campGrade = 0
	}

	// if it's a call again
	var count interface{}

	err = tx.Raw(`SELECT count(*) FROM vicidial_live_agents WHERE user = ? AND campaign_id = ?;`,
		param.Username, param.Campaign,
	).Row().Scan(&count)

	if err != nil {
		return err
	}

	fmt.Println("loggin entries : ", count, "count.(int64: ", count.(int64))

	// conference
	var (
		sipExt   = "SIP/" + phone.Extension
		callerID string
	)

	conf, err = calls.GetConference(tx, phone.ServerIP, sipExt)

	switch reflect.TypeOf(conf).String() {
	case "[]uint8":
		co, _ := strconv.Atoi(bytes.NewBuffer(conf.([]byte)).String())
		conf = int64(co)
	case "int64":
		conf = conf.(int64)
	}

	if err != nil {
		return err
	}

	// user wasn't already logged in
	if count.(int64) == 0 || (count != nil && count.(int64) > 1) {

		// delete all live agents
		err = tx.Exec(`
			DELETE FROM vicidial_live_agents WHERE user = ? AND campaign_id = ?;`,
			param.Username, param.Campaign,
		).Error

		if err != nil {
			return err
		}

		// insert a new live agent record
		callerID = fmt.Sprintf("S%v%v", timestamp, conf)

		err = tx.Exec(`
			INSERT INTO vicidial_live_agents 
			(
				user,server_ip,conf_exten,extension,status,lead_id,campaign_id,uniqueid,callerid,channel,random_id,
				agent_log_id, outbound_autodial,
				last_call_time,last_update_time,last_call_finish,closer_campaigns,user_level,campaign_weight,calls_today,
				last_state_change,manager_ingroup_set,on_hook_ring_time,on_hook_agent,last_inbound_call_time,
				last_inbound_call_finish,campaign_grade,pause_code,last_inbound_call_time_filtered,last_inbound_call_finish_filtered
			) 
			VALUES 
			(
				?, ?, ?, ?, 'PAUSED', 0, ?, '', '','', ?,
				?, "N",
				?, ?, ?, ?, ?, ?, ?,   
				?, ?, ?, ?, ?, 
				?, ?, ?, ?, ?
			)
		`, param.Username, phone.ServerIP, conf, sipExt, strings.ToUpper(param.Campaign), random,
			param.AgentLogID,
			nowTime, nowTime, nowTime, "AGENTDIRECT -", user.UserLevel, fmt.Sprintf("%d", campW), fmt.Sprintf("%v", callT),
			nowTime, "N", phone.PhoneRingTimeout, phone.OnHookAgent, nowTime,
			nowTime, fmt.Sprintf("%v", campGrade), "LOGIN", nowTime, nowTime,
		).Error

		if err != nil {
			return err
		}

		// user is logged in but phone disconnected
	} else if count != nil && count.(int64) == 1 {

		callerID = fmt.Sprintf("ACagcW%v%v", time.Now().Unix(), param.Username)

		err = tx.Exec(`
			UPDATE vicidial_live_agents SET last_update_time = NOW() WHERE user = ? AND extension = ?;`,
			param.Username, sipExt,
		).Error

		if err != nil {
			return fmt.Errorf("cannot clean up live user. %v", err)
		}
	}

	// 3. prepare manager data
	data := models.VicidialManager{
		EntryDate: time.Now(),
		Status:    manager.NEW,
		Response:  manager.NO,
		ServerIP:  phone.ServerIP,
		Channel:   "Channel: SIP/" + phone.Extension,
		Action:    "Originate",
		CallerID:  callerID,
		CmdLineB:  "Channel: SIP/" + phone.Extension,
		CmdLineC:  "Context: default",
		CmdLineD:  fmt.Sprintf("Exten: %v", conf),
		CmdLineE:  "Priority: 1",
		CmdLineF:  fmt.Sprintf("Callerid: \"%v\" <%s>", callerID, campCID),
	}

	if err = data.Save(tx); err != nil {
		return err
	}

	// update campaign
	err = tx.Exec(`UPDATE vicidial_campaigns SET campaign_logindate =  NOW() WHERE campaign_id = ?;`, param.Campaign).Error

	if err != nil {
		return err
	}

	// init a new web client session
	var sessionName = fmt.Sprintf("%v_%v%v", time.Now().Unix(), phone.Extension, random+10000000)

	err = tx.Exec(`
		INSERT INTO web_client_sessions VALUES (?, ?, ?, NOW(), ?);`,
		phone.Extension, phone.ServerIP, "agc",
		sessionName,
	).Error

	// insert session data
	err = tx.Exec(`
		INSERT INTO vicidial_session_data 
		SET 
			session_name = ?,
			user = ?,
			campaign_id = ?,
			server_ip = ?,
			conf_exten = ?,
			extension= ?,
			login_time = NOW(), 
			agent_login_call = ?;
	`, sessionName, param.Username, strings.ToUpper(param.Campaign), phone.ServerIP, conf, phone.Extension,
		fmt.Sprintf(`||%vNEW|N|%v||Originate|%v|%v|%v|%v|%v|%v|||||`,
			time.Now().Format("2006-01-02 15:04:00"), phone.ServerIP, callerID, data.Channel,
			data.CmdLineC, data.CmdLineD, data.CmdLineE, data.CmdLineF),
	).Error

	return err
}

// LogoutPhone - function to logout phone
func LogoutPhone(txn *gorm.DB, extension string) error {

	var (
		err         error
		phone       = new(models.Phone)
		liveSipChan interface{}
	)

	// 1. fetch phone
	if err = txn.Find(phone, "extension = ?", extension).Error; err != nil {
		return fmt.Errorf("phone login details not found %v", err)
	}

	err = txn.Raw(`
		SELECT channel FROM live_sip_channels 
		WHERE server_ip = ? AND channel LIKE ?`, phone.ServerIP, "SIP/"+phone.Extension+"%",
	).Row().Scan(&liveSipChan)

	// the phone isn't online so logout user only
	if err != nil && err == sql.ErrNoRows && liveSipChan == nil {
		return nil
	}

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// 2. Normal logout
	normalLogout := models.VicidialManager{
		EntryDate: time.Now(),
		Status:    manager.NEW,
		Response:  manager.NO,
		ServerIP:  phone.ServerIP,
		Channel:   "",
		Action:    "Hangup",
		CallerID:  fmt.Sprintf("ULGH3459%v", time.Now().Unix()),
		CmdLineB:  fmt.Sprintf("Channel: %s", liveSipChan),
		CmdLineC:  "",
		CmdLineD:  "",
		CmdLineE:  "",
		CmdLineF:  "",
	}

	if err = normalLogout.Save(txn); err != nil {
		return err
	}

	// 3. logout kick
	var conf interface{}

	err = txn.Raw(`
		SELECT conf_exten FROM vicidial_conferences 
		WHERE 
			extension = ? AND server_ip = ? LIMIT 1;`, "SIP/"+phone.Extension, phone.ServerIP,
	).Row().Scan(&conf)

	if err != nil {
		return fmt.Errorf("cannot find conference %v", err)
	}

	logoutKick := models.VicidialManager{
		EntryDate: time.Now(),
		Status:    manager.NEW,
		Response:  manager.NO,
		ServerIP:  "",
		Channel:   "",
		Action:    "Originate",
		CallerID:  fmt.Sprintf("ULGH3458%v", time.Now().Unix()),
		CmdLineB:  fmt.Sprintf("Local/5555%v@%v", conf, phone.ExtContext),
		CmdLineC:  "",
		CmdLineD:  "Exten: 8300",
		CmdLineE:  "",
		CmdLineF:  "",
	}

	return logoutKick.Save(txn)
}

// loginAgentWithTimeout - waits for agent to receive call
func loginAgentWithTimeout(phone string, duration int) error {

	var (
		err     error
		sipChan = fmt.Sprintf("SIP/%s", phone) + "%"
		channel interface{}
	)

	for range make([]int, duration) {

		// has call been picked?
		err = auth.DB.Raw(`SELECT channel FROM live_sip_channels WHERE channel LIKE ? AND extension != 'ring';`, sipChan).Row().Scan(&channel)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot query live channels. %v", err)
		}

		// yes
		if channel != nil {
			return nil
		}

		// sleep for a second
		time.Sleep(1 * time.Second)
	}

	return errors.New("login timeout")
}

func getPauseCodes(campaign string) ([]map[string]string, error) {

	var (
		codes = []map[string]string{}
		err   error
	)

	rows, err := auth.DB.Raw(`
		SELECT pause_code,pause_code_name FROM vicidial_pause_codes WHERE campaign_id = ?`, campaign,
	).Rows()

	if err != nil {
		return codes, err
	}

	for rows.Next() {

		var code, name interface{}

		if err = rows.Scan(&code, &name); err != nil {
			return codes, err
		}

		codes = append(codes, map[string]string{
			"name":       fmt.Sprintf("%s", name),
			"pause_code": fmt.Sprintf("%s", code),
		})
	}

	return codes, err
}
