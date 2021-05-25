package websocket

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"smartdial/controllers/calls"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

/*
SCENARIOS

	1. DIAL OUT (Outbound calls )
		- Decline call
		- Pick up call then hangup
		- Call autodialed, customer hangsup
		- Call from dialnext, customer hangs

	2. INBOUND CALL
		- Pick from queue, customer hangs-up
		- Autopicked (ready-mode), customer hangs

	3. TRANSFERED CALL
		- Customer hangs


	Due to inconsistency on how information of the above scenarios is presented,
	this is the effort to ensure all the possible scenarios is captured.

*/

// taken is a cache for storing auto_call_id
// since its' a global map, it stores autocallsid for all live users,
// when taken['user'] doesn't match autocallid for the particular user, the call has been dropped.
var taken = make(map[string]interface{})

// OngoingCall - status of an ongoing call
func (web *Action) OngoingCall(socket *websocket.Conn, user, phone string) {

	var (
		callerid, uid, comments, liveChan interface{}
		dialStatus, hangupReason          interface{}
		autoCallID, phoneNo               interface{}

		leadID, vacStatus, ingroup interface{}

		parked, grabbed, picked, autoPicked bool

		dead, closerDead bool

		channel, status, redirect interface{}
	)

	// 1. fetch user whose on a call
	err := func() error {

		// a. fetch lead id and callerid from live agent
		err := web.DB.Raw(`
			SELECT uniqueid, comments ,channel,callerid FROM vicidial_live_agents
			WHERE
				user = ? AND extension = ? AND
				(status = 'INCALL' OR status = 'QUEUE') AND
				(lead_id IS NOT NULL OR lead_id != 0) AND
				callerid IS NOT NULL;
		`, user, fmt.Sprintf("SIP/%v", phone),
		).Row().Scan(&uid, &comments, &channel, &callerid)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// b. fetch hungup reason and call status
		if callerid != nil {

			err = web.DB.Raw(`
				SELECT dialstatus, sip_hangup_reason FROM vicidial_carrier_log
				WHERE caller_code = ?;
			`, callerid).Row().Scan(&dialStatus, &hangupReason)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			if strings.Contains(fmt.Sprintf("%s", callerid), "V") &&
				fmt.Sprintf("%s", dialStatus) == "ANSWER" {
				autoPicked = true
			}

		}

		// c. check if picked up - livechannel contains a value if the call was indeed picked
		if dialStatus == nil && callerid != nil {

			err = web.DB.Raw(`
				SELECT channel FROM vicidial_manager
				WHERE callerid = ?;
			`, callerid).Row().Scan(&liveChan)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// update live agent to incall. some calls this is done automatic
			if liveChan != nil && callerid != nil {

				err = web.DB.Exec(`
					UPDATE vicidial_auto_calls SET channel = ?
					WHERE callerid = ?;`, liveChan, callerid,
				).Error

				if err != nil {
					return err
				}

			}

			// livechannel can also be found in the auto calls
			if liveChan == nil && callerid != nil {

				err = web.DB.Raw(`
					SELECT channel FROM vicidial_auto_calls
					WHERE callerid = ?;
				`, callerid).Row().Scan(&liveChan)

				if err != nil && err != sql.ErrNoRows {
					return err
				}
			}

			// check if livechann contains "SIP".
			if liveChan != nil {

				picked = strings.Contains(fmt.Sprintf("%s", bytes.NewBuffer(liveChan.([]byte)).String()), "SIP")

				// redirect action
				err = web.DB.Raw(`SELECT action FROM vicidial_manager WHERE cmd_line_b = ?;`,
					fmt.Sprintf("Channel: %s", liveChan)).Row().Scan(&redirect)

				if err != nil && err != sql.ErrNoRows {
					return err
				}
			}

		}

		// d. check taken call (from the queue) if it has hangup
		if uid != nil {

			err = web.DB.Raw(`
				SELECT auto_call_id, status, lead_id, phone_number FROM vicidial_auto_calls
				WHERE uniqueid = ? OR callerid = ?;
			`, uid, callerid).Row().Scan(&autoCallID, &vacStatus, &leadID, &phoneNo)

			if err != nil && err != sql.ErrNoRows {
				return err
			}
		}

		// e. inbound call - an incoming call that is auto-picked. We grab the details and send them to client
		if vacStatus != nil && fmt.Sprintf("%s", vacStatus) == "CLOSER" {

			err = web.DB.Raw(`
				SELECT
					vac.auto_call_id, vac.campaign_id, vac.lead_id, vac.callerid, vac.phone_number
				FROM vicidial_auto_calls vac
				INNER JOIN vicidial_live_agents vla ON
					vac.uniqueid = vla.uniqueid
				WHERE
					vac.status = 'CLOSER' AND
					vla.status = 'INCALL' AND
					vac.callerid = vla.callerid AND
					vac.lead_id = vla.lead_id;
			`).Row().Scan(&autoCallID, &ingroup, &leadID, &callerid, &phoneNo)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			if autoCallID != nil && callerid != nil {
				autoPicked = true
			}

		}

		// this is for comparison later on when we determine a call taken from the queue has hangedup
		// we know a call has hangup when it disappers from the table so we'll check if `autoCallID != taken`
		// AutocallID -  format auto call id which can be bytes or int64 interchangerbly
		if autoCallID != nil {

			switch reflect.TypeOf(autoCallID).String() {
			case "[]uint8":
				aid, _ := strconv.Atoi(bytes.NewBuffer(autoCallID.([]byte)).String())
				autoCallID = int64(aid)
			case "int64":
				autoCallID = autoCallID.(int64)
			}

			taken[user] = autoCallID
		}

		// f. check if parked/grabbed. a call in either state shouldn't hangup
		if callerid != nil {

			var parkGrab interface{}

			// parked/grabbed
			err = web.DB.Raw(`SELECT status FROM park_log WHERE extension = ?;`, callerid).Row().Scan(&parkGrab)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			if parkGrab != nil {

				if fmt.Sprintf("%s", parkGrab) == "PARKED" {
					parked = true
				} else if fmt.Sprintf("%s", parkGrab) == "GRABBED" {
					grabbed = true
				}
			}

			// (A) status - specifically we are interested in a dead call status dead call
			err = web.DB.Raw(`SELECT status FROM vicidial_manager WHERE callerid = ?;`, callerid).Row().Scan(&status)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// check if call is dead
			if status != nil {

				fmt.Println("call status : ", status)

				dead = fmt.Sprintf("%s", status) == "DEAD"

				fmt.Println("DEAD: ", fmt.Sprintf("%s", status), dead)

			}

			// (B) check for closer dead
			var closerStat interface{}

			if channel != nil {

				err = web.DB.Raw(`SELECT status FROM vicidial_manager WHERE channel = ?;`, channel).Row().Scan(&closerStat)

				if err != nil && err != sql.ErrNoRows {
					return err
				}

				// check if call is dead
				if closerStat != nil {

					fmt.Println("closer status : ", closerStat)

					closerDead = fmt.Sprintf("%s", closerStat) == "DEAD"

					fmt.Println("CLOSER DEAD: ", fmt.Sprintf("%s", closerStat), closerDead)
				}
			}

		}

		// DEAD --- set dial status
		if dialStatus == nil && (closerDead || dead || taken[user] != autoCallID) {
			dialStatus = "DEAD"
		}

		return nil
	}()

	// 2. error occurred
	if err != nil {
		payload := map[string]interface{}{
			"error": fmt.Sprintf("cannot check ongoing call status. %v", err),
		}
		socket.WriteJSON(payload)
		web.Logger.Errorf("%s", payload["error"])
	}

	// 3. respond to socket

	fmt.Println("Call Type : ", comments, " ", fmt.Sprintf("%s", comments))

	fmt.Println("dial status : ", dialStatus, " ", fmt.Sprintf("%s", dialStatus))

	fmt.Println("hangup reason : ", hangupReason, " ", fmt.Sprintf("%s", hangupReason))

	fmt.Println("Auto-picked : ", autoPicked)

	println("Autodial picked = ", strings.Contains(fmt.Sprintf("%s", callerid), "V") && fmt.Sprintf("%s", dialStatus) == "ANSWER")

	fmt.Println("Agent picked : ", picked)

	fmt.Println("Call parked : ", parked)

	fmt.Println("Call grabbed : ", grabbed)

	fmt.Printf("Taken : %v AutocallID : %v\n", taken[user], autoCallID)

	fmt.Printf("Taken == AutoCallID :  %v\n", taken[user] == autoCallID)

	// 1. DIALING ...
	// call hasn't been picked yet
	if dialStatus == nil && liveChan != nil && !picked && parked != true {

		fmt.Println("Dialing ...")

		payload := map[string]interface{}{
			"dial_status": "calling",
		}

		socket.WriteJSON(payload)

		// 2. LIVECALL
		// call picked - call was picked by customer or by agent from queue or autocall
	} else if (liveChan != nil && callerid != nil) || (picked || autoPicked) {

		fmt.Println("Livecall ...")

		payload := map[string]interface{}{
			"dial_status": "livecall",
		}

		// there are phone details to be passed to the client
		// autopicked call - agent need information about the connected line
		if leadID != nil || phoneNo != nil {

			var callDetails = map[string]interface{}{}

			// autopicked - picked
			if autoPicked {
				callDetails["callerid"] = autoCallID
			} else if picked {
				callDetails["callerid"] = fmt.Sprintf("%s", callerid)
			}

			// lead_id - could be int64 or []uint8
			if lID, ok := leadID.(int64); !ok {
				callDetails["lead_id"] = bytes.NewBuffer(leadID.([]byte)).String()
			} else {
				callDetails["lead_id"] = lID
			}

			callDetails["phone_number"] = fmt.Sprintf("%s", phoneNo)

			// get url for the iframe
			if ingroup != nil {
				url, _ := calls.GetScriptURL(web.DB, "in", fmt.Sprintf("%s", phoneNo), fmt.Sprintf("%s", ingroup))
				callDetails["url"] = url
			}

			if comments != nil {
				//callDetails["call_type"] = comments
				callDetails["call_type"] = fmt.Sprintf("%s", comments)
			}

			fmt.Println("URL : ", callDetails["url"])

			payload["call_details"] = callDetails
		}

		socket.WriteJSON(payload)

	}

	// 3. HANGUP or REDIRECT
	// hangup -  call has hanged up
	// a. there's a dial status or a hangup reason or
	// b. id mismatch if the call was taken from queue and the call isn't parked
	if dialStatus != nil && (!picked || !autoPicked) && (!parked && !grabbed) && (status != nil || redirect != nil) || autoCallID != taken[user] || dead || closerDead {

		fmt.Println("Hangup !")

		var (
			reason = fmt.Sprintf("%s", hangupReason)

			payload = map[string]interface{}{}
		)

		// (A) call has hangup because it was transfered
		if redirect != nil && bytes.NewBuffer(redirect.([]byte)).String() == "Redirect" {

			payload = map[string]interface{}{
				"dial_status": "transfer",
				"code":        "XFER",
			}

		}

		// (B) call hanged up
		if dead || closerDead || autoCallID != taken[user] && dialStatus != nil {

			payload = map[string]interface{}{
				"dial_status": "hangup",
			}

			if hangupReason != nil {

				// remove trailing parenthesis from hangup reason
				if strings.Contains(reason, ")") {
					reason = reason[:strings.LastIndex(reason, ")")]
				}

				payload["code"] = fmt.Sprintf("%s", dialStatus)
				payload["hangup_reason"] = reason

			} else {
				payload["code"] = "HANGUP"
				payload["hangup_reason"] = "Call Ended"
			}

			delete(taken, user)
		}
		fmt.Println(payload)

		socket.WriteJSON(payload)
	}

}
