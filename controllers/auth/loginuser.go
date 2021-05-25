package auth

import (
	"fmt"
	"net/http"
	"smartdial/data"
	"smartdial/log"
	"smartdial/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

var auth = &Auth{
	Logger: log.GetLogger(),
	DB:     data.GetDB(),
}

type ulogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type plogin struct {
	Campaign   string `json:"campaign"`
	Username   string `json:"username"`
	AgentLogID int64
}

type ulogout struct {
	Username string `json:"username"`
	// Phone    string `json:"phone"`
}

type changecamp struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Campaign string `json:"campaign"`
}

// LoginUser -
func LoginUser(c *gin.Context) {

	var (
		token     string
		err       error
		campaigns []string
		cred      = new(ulogin)
	)

	// 1. parse request
	if err = c.Bind(cred); err != nil {
		auth.Logger.Errorf("cannot parse login user details : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprint(err),
		})
		c.Abort()
		return
	}

	fmt.Printf("login user : %+v\n", cred)

	err = auth.DB.Transaction(func(tx *gorm.DB) error {

		// 2. authenticate user
		token, err = auth.SignIn(cred.Username, cred.Password)

		if err != nil {
			// failed login attempt  + 1
			// use auth.DB instead of tx otherwhise it'll rollback this update
			if e := auth.DB.Exec(`
				UPDATE vicidial_users 
				SET 
					failed_login_count = (failed_login_count + 1),
					last_login_date = IF(failed_login_count >= 10, last_login_date, NOW())
				WHERE 
					user = ? ;`, cred.Username,
			).Error; e != nil {
				auth.Logger.Errorf("cannot update vcusers on failed login attempt user %v : error : %v", cred.Username, e)
			}

			return err
		}

		// 3. Save token to cache
		tx.Exec(`DELETE FROM tokens WHERE username = ?;`, cred.Username)
		tx.Exec(`INSERT INTO tokens (username, token) VALUES (?, ?);`, cred.Username, token)

		// 4.fetch campaigns
		var ugroup, allowed interface{}
		// check user group
		tx.Raw(`SELECT user_group FROM vicidial_users WHERE user = ?`, cred.Username).Row().Scan(&ugroup)
		// check allowed campaigns for the user group
		if err = tx.Raw(`SELECT allowed_campaigns FROM vicidial_user_groups WHERE user_group = ?`, ugroup).Row().Scan(&allowed); err != nil {
			return err
		}

		if strings.Contains(fmt.Sprintf("%s", allowed), "ALL-CAMPAIGNS") {
			// select all distinct campaigns
			query := `SELECT campaign_id FROM vicidial_campaigns GROUP BY campaign_id;`

			rows, err := tx.Raw(query).Rows()

			if err != nil {
				return err
			}

			for rows.Next() {

				var camp interface{}

				if err = rows.Scan(&camp); err != nil {
					return err
				}

				campaigns = append(campaigns, fmt.Sprintf("%s", camp))
			}

		} else {

			for _, c := range strings.Split(fmt.Sprintf("%s", allowed), " ") {

				if len(c) > 1 {
					campaigns = append(campaigns, c)
				}
			}

		}

		// 5. update to logged in
		err = tx.Exec(`
			UPDATE vicidial_users 
			SET 
				failed_login_count = 0, 
				last_login_date = NOW(), 
				last_ip = ?
			WHERE 
				user = ? AND 
				pass = ?;`, utils.GetLocalIP(), cred.Username, cred.Password,
		).Error

		return err
	})

	// 6. error response
	if err != nil {
		auth.Logger.Errorf("failed to login : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprint(err),
		})
		c.Abort()
		return
	}

	// 7. successful login
	c.JSON(http.StatusOK, gin.H{
		"status":    http.StatusText(http.StatusOK),
		"user":      cred.Username,
		"token":     token,
		"campaigns": campaigns,
	})
	c.Abort()
	return
}
