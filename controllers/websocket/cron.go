package websocket

import (
	"math/rand"
	"time"
)

// KeepAlive -
func KeepAlive(params *Data) {

	var (
		random = rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1000000)
	)

	err := web.DB.Exec(`
		UPDATE vicidial_live_agents 
		SET 
			status = IF(status ='QUEUE', 'INCALL', status),
			random_id = ?, 
			last_update_time = NOW() 
		WHERE 
			user = ? AND 
			campaign_id = ?;
	`, random, params.Username, params.Campaign).Error

	if err != nil {
		web.Logger.Errorf("[KEEP ALIVE] cannot update live agent. %v", err)
	}
}
