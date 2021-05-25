package routes

import (
	"net/http"
	"os"
	"smartdial/config"
	"smartdial/controllers/auth"
	"smartdial/controllers/calls"
	"smartdial/controllers/transfer"
	"smartdial/controllers/websocket"
	middleware "smartdial/middlewares"

	"github.com/gin-gonic/gin"
)

var conf = config.GetConfig()

// Router - returns gin router engine
func Router() *gin.Engine {

	var ENV = os.Getenv("APP_ENV")

	// If we're in production mode, set Gin to "release" mode
	if ENV == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// 1. Authentication and Authorization
	v1 := router.Group("/api/v1/")
	v1.Use(middleware.CORS())

	// test API endpoints
	v1.GET("/foo", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"foo": "bar"})
		c.Abort()
	})

	v1.OPTIONS("/login", auth.LoginUser)
	v1.POST("/login", auth.LoginUser)

	// 2. Auth required
	v2 := router.Group("/api/v1/")
	v2.Use(middleware.CORS())
	v2.Use(middleware.Auth())

	/*
		CAMPAIGN/INGROUP OPERATION
	*/

	v2.OPTIONS("/campaign/login", auth.LoginToCampaign)
	v2.POST("/campaign/login", auth.LoginToCampaign)

	v2.OPTIONS("/campaign/switch", auth.SwitchCampaign)
	v2.POST("/campaign/switch", auth.SwitchCampaign)

	v2.OPTIONS("/logout", auth.Logout)
	v2.POST("/logout", auth.Logout)

	v2.OPTIONS("/closer/inbound", calls.InboundGroup)
	v2.POST("/closer/inbound", calls.InboundGroup)

	/*
		DIAL OPERATION
	*/

	// manual dial
	v2.OPTIONS("/dial/manual", calls.ManualDial)
	v2.POST("/dial/manual", calls.ManualDial)

	// delete call back
	v2.OPTIONS("/dial/deleteCallback", calls.DeleteCallBack)
	v2.POST("/dial/deleteCallback", calls.DeleteCallBack)

	// hangup
	v2.OPTIONS("/dial/hangup", calls.HangUpCall)
	v2.POST("/dial/hangup", calls.HangUpCall)

	// dispose call
	v2.OPTIONS("/dial/dispose", calls.DisposeCall)
	v2.POST("/dial/dispose", calls.DisposeCall)

	// next dial
	v2.OPTIONS("/dial/next", calls.DialNext)
	v2.POST("/dial/next", calls.DialNext)

	// park call
	v2.OPTIONS("/dial/park", calls.ParkCall)
	v2.POST("/dial/park", calls.ParkCall)

	// grab call
	v2.OPTIONS("/dial/grab", calls.GrabParkedCall)
	v2.POST("/dial/grab", calls.GrabParkedCall)

	// take call
	v2.OPTIONS("/dial/take", calls.TakeCall)
	v2.POST("/dial/take", calls.TakeCall)

	// send DTMF
	v2.OPTIONS("/dial/dtmf", calls.SendDTMF)
	v2.POST("/dial/dtmf", calls.SendDTMF)

	/*
		AGENT STATUS OPERATION
	*/

	// toggle pause - active
	v2.OPTIONS("/dial/status", calls.TogglePause)
	v2.POST("/dial/status", calls.TogglePause)

	// switch pause code
	v2.OPTIONS("/dial/pause-code-switch", calls.SwitchPauseCode)
	v2.POST("/dial/pause-code-switch", calls.SwitchPauseCode)

	// call logs -
	v2.OPTIONS("/dial/logs", calls.FetchLogs)
	v2.POST("/dial/logs", calls.FetchLogs)

	// dialable leads -
	v2.OPTIONS("/dial/leads", calls.LeadsCount)
	v2.POST("/dial/leads", calls.LeadsCount)

	//users -
	v2.OPTIONS("/dial/users", calls.GetAllUsers)
	v2.POST("/dial/users", calls.GetAllUsers)

	//users -
	v2.OPTIONS("/dial/callbacks", calls.GetAllCallBacks)
	v2.POST("/dial/callbacks", calls.GetAllCallBacks)
	/*
		CALL TRANSFER OPERATION
	*/

	// transfer local
	v2.OPTIONS("/transfer/local", transfer.LocalCloser)
	v2.POST("/transfer/local", transfer.LocalCloser)

	// transfer add call
	v2.OPTIONS("/transfer/add", transfer.AddCall)
	v2.POST("/transfer/add", transfer.AddCall)

	// transfer with customer on hold
	v2.OPTIONS("/transfer/park", transfer.AddCallCustomerParked)
	v2.POST("/transfer/park", transfer.AddCallCustomerParked)

	// leave the 3way
	v2.OPTIONS("/transfer/leave3way", transfer.Leave3Way)
	v2.POST("/transfer/leave3way", transfer.Leave3Way)

	// hangup all channels
	v2.OPTIONS("/transfer/hangall", calls.HangUpCall)
	v2.POST("/transfer/hangall", calls.HangUpCall)

	// hangup xfer line
	v2.OPTIONS("/transfer/hangxfer", transfer.HangupXFERLine)
	v2.POST("/transfer/hangxfer", transfer.HangupXFERLine)

	// hangup customer
	v2.OPTIONS("/transfer/hang-customer", transfer.HangupXFERLine)
	v2.POST("/transfer/hang-customer", transfer.HangupXFERLine)

	// external blind transfer
	v2.OPTIONS("/transfer/blind", transfer.LocalCloser)
	v2.POST("/transfer/blind", transfer.LocalCloser)

	v1.GET("/ws", websocket.WebSocket)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"message": "Sorry! That one is not handled Here"})
	})

	return router
}
