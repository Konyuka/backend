package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"smartdial/data"
	"smartdial/log"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type (
	// Action -
	Action struct {
		DB     *gorm.DB
		Logger *logrus.Logger
	}

	// Data -
	Data struct {
		Username string `json:"username"`
		Phone    string `json:"phone"`
		Campaign string `json:"campaign"`
	}
)

// declare upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var web = &Action{
	DB:     data.GetDB(),
	Logger: log.GetLogger(),
}

// WebSocket -
func WebSocket(c *gin.Context) {

	// 1. instantiate ws conn
	socket, err := upgrader.Upgrade(c.Writer, c.Request, nil)

	if err != nil {
		web.Logger.Errorf("cannot establish websocket connection. %v", err)
	}

	fmt.Printf("Websocket client successfuly connected.\n")

	_, p, err := socket.ReadMessage()

	if err != nil {
		web.Logger.Errorf("[WEBSOCKET] cannot read message from websocket. %v", err)
		socket.WriteJSON(`{"error": "cannot read message payload"}`)
		return
	}

	//decode payload ...
	var info = new(Data)

	if err = json.Unmarshal(p, info); err != nil {
		web.Logger.Errorf("[WEBSOCKET] cannot parse request. %v", err)
		socket.WriteJSON(`{"error": "cannot parse request"}`)
		return
	}

	fmt.Printf("incoming data : %+v\n", info)

	// 3. sending data to client
	if len(info.Username) > 0 && len(info.Phone) > 0 {

		for {

			// a. user still logged in?
			if ok := web.isLoggedToCampaign(socket, info); !ok {
				defer socket.Close()
				//break
			}

			// b. calls on queue
			web.Queue(socket, info)

			// b. incall status
			web.OngoingCall(socket, info.Username, info.Phone)

			// c. keep user alive
			KeepAlive(info)
		}
	}

}
