package calls

import (
	//"errors"
	"bytes"
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/uniplaces/carbon"
	"net/http"
	"smartdial/data"
	"smartdial/models"
)

// GetAllUserGroups - User groups to be used when creating a new user
func GetAllCallBacks(c *gin.Context) {
	var (
		db        = data.GetDB()
		params    = new(models.User)
		err       error
		callBacks []models.VicidialCallback
	)

	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse callbacks request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	err = func() error {

		// a. select agents who are active
		rows, err := db.Raw(`
			SELECT
				callback_id, lead_id, campaign_id, entry_time, callback_time, comments, customer_time
			FROM vicidial_callbacks
			WHERE
				callback_time BETWEEN ? AND (NOW() + INTERVAL 1 Hour) AND
				recipient = 'ANYONE' OR (recipient = 'USERONLY' AND user = ?) AND
				(status = 'ACTIVE' OR status = 'LIVE')
			ORDER BY callback_time DESC;`, carbon.Now().StartOfWeek().String(), params.User,
		).Rows()

		if err != nil && err != sql.ErrNoRows {
			return err
		}
		for rows.Next() {

			var (
				cb      = models.VicidialCallback{}
				phoneNo interface{}
			)

			// scan values
			if err = rows.Scan(
				&cb.CallbackID, &cb.LeadID, &cb.CampaignID, &cb.EntryTime,
				&cb.CallbackTime, &cb.Comments, &cb.CustomerTime,
			); err != nil && err != sql.ErrNoRows {
				return err
			}

			// get the phone number
			err = db.Raw(`SELECT phone_number FROM vicidial_list WHERE lead_id = ?;`, cb.LeadID).Row().Scan(&phoneNo)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			if err == nil && phoneNo != nil {
				cb.PhoneNumber = bytes.NewBuffer(phoneNo.([]byte)).String()

				callBacks = append(callBacks, cb)
			}
		}

		return nil

	}()

	if err != nil {
		call.Logger.Errorf("[FETCH CALL BACKS] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    http.StatusText(http.StatusOK),
		"callbacks": callBacks,
	})
	c.Abort()
	return
}
