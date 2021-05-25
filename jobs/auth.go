package jobs

import (
	"database/sql"
	"fmt"
	"smartdial/data"
	"smartdial/log"
	"smartdial/models"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// CronJob -
type CronJob struct {
	DB     *gorm.DB
	Logger *logrus.Logger
}

// NewCronJob - instantiates CronJob
func NewCronJob() *CronJob {
	return &CronJob{
		DB:     data.GetDB(),
		Logger: log.GetLogger(),
	}
}

// LogoutInactive - logs user out if asterisk not active
func (c *CronJob) LogoutInactive() {

	var (
		err   error
		phone = new(models.Phone)
	)

	// begin a transaction
	err = func() error {

		// 1. get all in live agents
		rows, err := c.DB.Raw(`SELECT user FROM vicidial_live_agents WHERE NOW() > (last_call_time + INTERVAL 30 second);`).Rows()

		if err != nil && err != sql.ErrNoRows {
			c.Logger.Errorf("cannot select users from live agents table. %v", err)
			return err
		}

		users, err := scanRowsToArray(rows)

		if len(users) == 0 {
			return nil
		}

		var ra interface{}

		err = c.DB.Raw(`SELECT COUNT(username) FROM tokens WHERE username IN (?);`, users).Row().Scan(&ra)

		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("cannot query tokens table. %v", err)
		}

		if ra == nil || ra.(int64) == 0 {
			return nil
		}

		// 2. check if returned users are online
		for _, user := range users {

			// a. fetch extension from users
			var extension interface{}

			err = c.DB.Raw(`
				SELECT phone_login FROM vicidial_users 
				WHERE user = ? AND phone_login IS NOT NULL AND phone_login <> '';`, user,
			).Row().Scan(&extension)

			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("cannot find phone login for user %v. %v", user, err)
			}

			// b. fetch server_ip from phones
			if err = c.DB.Find(phone, "extension = ?", extension).Error; err != nil {
				return fmt.Errorf("cannot find phone record for %v. %v", extension, err)
			}

			// c. check if phone  is logged to campaign
			var (
				sipExt  = "SIP/" + phone.Extension
				channel interface{}
			)

			err = c.DB.Raw(`SELECT channel FROM live_sip_channels WHERE server_ip = ? AND channel LIKE ?;`,
				phone.ServerIP, sipExt+"%").Row().Scan(&channel)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			if err == sql.ErrNoRows {
				return nil
			}

			// d. revoke user token
			if channel == nil {
				err = c.DB.Exec(`DELETE FROM tokens WHERE username = ?;`, user).Error
				if err != nil {
					return fmt.Errorf("cannot delete user token %v. %v", user, err)
				}
			}
		}

		return err
	}()

	// 3. handle errors if any
	if err != nil {
		c.Logger.Errorf("cannot clean up. %v", err)
	}
}

// scanRowsToArray - scans sql rows to slice of string
func scanRowsToArray(rows *sql.Rows) ([]string, error) {

	var users []string

	for rows.Next() {

		var u interface{}

		if err := rows.Scan(&u); err != nil {
			return users, err
		}

		users = append(users, fmt.Sprintf("%s", u))
	}

	return users, nil
}
