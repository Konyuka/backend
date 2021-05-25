package calls

import (
	//"errors"
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"smartdial/models"
)

// GetAllUserGroups - User groups to be used when creating a new user
func GetAllUsers(c *gin.Context) {
	var (
		tx         = call.DB.Begin()
		params     = new(models.User)
		err        error
		liveAgents []map[string]string
	)

	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse users request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	err = func() error {

		// a. select agents who are active
		rows, err := tx.Raw(`SELECT user, status, campaign_id FROM vicidial_live_agents WHERE user != ?;`, params.User).Rows()

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// prepare array
		for rows.Next() {

			var agent, userStatus, campaign interface{}

			if err = rows.Scan(&agent, &userStatus, &campaign); err != nil && err != sql.ErrNoRows {
				return err
			}

			liveAgents = append(liveAgents, map[string]string{
				"user":     fmt.Sprintf("%s", agent),
				"status":   fmt.Sprintf("%s", userStatus),
				"campaign": fmt.Sprintf("%s", campaign),
			})
		}

		return nil

	}()

	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[FETCH ACTIVE USERS] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusText(http.StatusOK),
		"agents": liveAgents,
	})
	c.Abort()
	return
}
