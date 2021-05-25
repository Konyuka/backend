package websocket

import (
	"bytes"
	"database/sql"
	"fmt"
	"smartdial/utils"

	"github.com/gorilla/websocket"
)

// Queue - display calls on queue
func (web *Action) Queue(socket *websocket.Conn, info *Data) {

	var (
		ingroups []string
		queue    []map[string]string
	)

	// fetch calls on queue transaction
	err := func() error {

		// a. select ingroups subscribed by users
		rows, err := web.DB.Raw(`SELECT group_id FROM vicidial_live_inbound_agents WHERE user = ?;`, info.Username).Rows()

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		var group interface{}

		// process rows
		for rows.Next() {

			if err = rows.Scan(&group); err != nil && err != sql.ErrNoRows {
				return err
			}

			val := fmt.Sprintf("'%s'", group)

			if val != "AGENTDIRECT" {
				ingroups = append(ingroups, val)
			}

		}

		// b. select calls on queue for this user
		if len(ingroups) < 1 {
			return nil
		}

		query := fmt.Sprintf(`
			SELECT 
				lead_id, campaign_id, phone_number, 
				(UNIX_TIMESTAMP(NOW()) - UNIX_TIMESTAMP(call_time)) AS duration, auto_call_id 
			FROM vicidial_auto_calls 
			WHERE 
				status IN('LIVE', 'CLOSER') AND 
				(campaign_id IN %v AND agent_only = '')  OR 
				(agent_only = '%v' AND campaign_id = 'AGENTDIRECT')
			ORDER BY queue_priority,call_time;`, utils.SQLINify(ingroups), info.Username)

		rows, err = web.DB.Raw(query).Rows()

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		for rows.Next() {

			var leadID, ingroup, phoneNumber, duration, callID interface{}

			if err = rows.Scan(&leadID, &ingroup, &phoneNumber, &duration, &callID); err != nil && err != sql.ErrNoRows {
				return err
			}

			queue = append(queue,
				map[string]string{
					"lead_id":    bytes.NewBuffer(leadID.([]byte)).String(),
					"ingroup":    bytes.NewBuffer(ingroup.([]byte)).String(),
					"caller":     bytes.NewBuffer(phoneNumber.([]byte)).String(),
					"queue_time": bytes.NewBuffer(duration.([]byte)).String(),
					"call_id":    bytes.NewBuffer(callID.([]byte)).String(),
				},
			)
		}

		return nil
	}()

	if err != nil {
		socket.WriteJSON(`{"error": "cannot find ingroups for user"}`)
		web.Logger.Errorf("[QUEUE] cannot find ingroups for user %v", info.Username)
	}

	fmt.Println("calls on queue\n", queue)

	if err == nil {
		socket.WriteJSON(map[string]interface{}{
			"queue": queue,
		})
	}

}
