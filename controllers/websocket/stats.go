package websocket

import (
	"github.com/gorilla/websocket"
	"smartdial/utils"
	"time"
)

// IsLoggedToCampaign - checks if a campaign logged into. returns true or false
func (web *Action) isLoggedToCampaign(socket *websocket.Conn, info *Data) bool {

	//time.Sleep(2200 * time.Minute)
	time.Sleep(2200 * time.Millisecond)

	// a. check whether there's an open channel for this user
	ok := func() bool {

		var channel interface{}

		err := web.DB.Raw(`SELECT channel FROM live_sip_channels WHERE server_ip = ? AND channel LIKE ?;`, utils.GetLocalIP(), "SIP/"+info.Phone+"%").Row().Scan(&channel)

		if err != nil || channel == nil {
			return false
		}
		return true
	}()

	payload := map[string]interface{}{"isloggedin": ok}

	socket.WriteJSON(payload)
	return ok
}
