package models

import (
	"fmt"
	"smartdial/models/constants/manager"
	"time"

	"github.com/jinzhu/gorm"
)

// VicidialManager - vicidial_manager table
type VicidialManager struct {
	ManID     int              `json:"man_id"`
	UniqueID  string           `json:"uniqueid"`
	EntryDate time.Time        `json:"entry_date"`
	Status    manager.Status   `json:"status"`
	Response  manager.Response `json:"response"`
	ServerIP  string           `json:"server_ip"`
	Channel   string           `json:"channel"`
	Action    string           `json:"action"`
	CallerID  string           `json:"callerid"`
	CmdLineB  string           `json:"cmd_line_b"`
	CmdLineC  string           `json:"cmd_line_c"`
	CmdLineD  string           `json:"cmd_line_d"`
	CmdLineE  string           `json:"cmd_line_e"`
	CmdLineF  string           `json:"cmd_line_f"`
	CmdLineG  string           `json:"cmd_line_g"`
	CmdLineH  string           `json:"cmd_line_h"`
	CmdLineI  string           `json:"cmd_line_i"`
	CmdLineJ  string           `json:"cmd_line_j"`
	CmdLineK  string           `json:"cmd_line_k"`
}

// Save - new entry to vicidial_manager
func (m VicidialManager) Save(db *gorm.DB) error {

	// man_id, unique_id, entry_date, status, response, server_ip, channel, action, caller_id, cmd_line_b, cmd_line_c, cmd_line_d, cmd_line_e, cmd_line_f
	query := fmt.Sprintf(`INSERT INTO vicidial_manager 
		VALUES
			(0,'','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v','%v')`,
		m.EntryDate.Format("20060102150405"), m.Status, m.Response, m.ServerIP, m.Channel, m.Action, m.CallerID, m.CmdLineB, m.CmdLineC, m.CmdLineD, m.CmdLineE,
		m.CmdLineF, m.CmdLineG, m.CmdLineH, m.CmdLineI, m.CmdLineJ, m.CmdLineK,
	)

	if err := db.Exec(query).Error; err != nil {
		return err
	}

	return nil
}
