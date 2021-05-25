package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Logout - request logs out user and phone
func Logout(c *gin.Context) {

	var (
		err   error
		param = new(ulogout)
		phone interface{}
		tx    = auth.DB.Begin()
	)

	// 1. parse request data
	if err = c.Bind(param); err != nil {
		auth.Logger.Errorf("cannot parse request for user logout: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("error : %v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("logout user : %+v\n", param)

	err = func() error {

		//  2. pick up phone_login
		err = tx.Raw(`
			SELECT phone_login FROM vicidial_users WHERE  user = ?;`, param.Username).Row().Scan(&phone)

		// error unrelated to `no rows returned`
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// phone is empty or no rows returned
		if phone == nil || err == sql.ErrNoRows {
			return errors.New("user doesn't have a phone login")
		}

		// 2. log out phone
		extension := fmt.Sprintf("%s", phone)
		if err = LogoutPhone(tx, extension); err != nil {
			return err
		}

		// 3. delete from live agent
		err = tx.Exec(`
			DELETE FROM vicidial_live_agents WHERE user = ? AND extension = ?;`,
			param.Username, "SIP/"+extension,
		).Error

		if err != nil {
			return err
		}

		// 4. delete from inbound group
		err = tx.Exec(`DELETE FROM vicidial_live_inbound_agents WHERE user = ?;`, param.Username).Error
		if err != nil {
			return err
		}

		// 5. delete from closer log
		err = tx.Exec(`DELETE FROM vicidial_user_closer_log WHERE user = ?;`, param.Username).Error
		if err != nil {
			return err
		}

		// 5. Update vicidial users
		err = tx.Exec(`UPDATE vicidial_users SET closer_campaigns = NULL WHERE user = ?;`, param.Username).Error
		if err != nil {
			return err
		}

		// 6. delete user token
		if err = tx.Exec(`DELETE FROM tokens WHERE username = ?;`, param.Username).Error; err != nil {
			return err
		}

		// 7. delete active web session
		if err = tx.Exec(`DELETE FROM web_client_sessions WHERE extension = ?;`, phone).Error; err != nil {
			return err
		}

		// 8. delete session data
		err = tx.Exec(`DELETE FROM vicidial_session_data WHERE user = ? AND extension = ?;`, param.Username, phone).Error

		return err
	}()

	// 9. error response
	if err != nil {
		tx.Rollback()
		auth.Logger.Errorf("cannot parse request : %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("logout phone failed. %v", err),
		})
		c.Abort()
		return
	}

	// 10. success logged out
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusText(http.StatusOK),
		"message": "logout successful",
	})
	c.Abort()
	return
}
