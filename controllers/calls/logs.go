package calls

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"smartdial/models"
)

//sort in order

type callLogs struct {
}

// FetchLogs -
func FetchLogs(c *gin.Context) {

	var (
		params = new(callogs)
		err    error

		tx = call.DB.Begin()

		callLogs []map[string]interface{}
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse fetch call logs request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	fmt.Printf("callogs payload : %+v\n", params)

	// 2. send the details
	err = func() error {

		// a. get phone details
		var phone = new(models.Phone)

		err = tx.Raw(`SELECT * FROM phones WHERE extension = ? LIMIT 1;`, params.Phone).Scan(&phone).Error

		if err != nil {
			return fmt.Errorf("cannot find agent's phone details. %v", err)
		}

		var BeginDate = params.Date + " 0:00:00"
		var EndDate = params.Date + " 23:59:59"

		//get outbound calls
		rows, err := tx.Raw(`
			SELECT
				call_date,
				phone_number,
				status
			FROM vicidial_log
			WHERE
				user=? and call_date >= ?  and call_date <=? order by call_date desc limit 10000
		`, params.Username, BeginDate, EndDate).Rows()

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// iterate rows preparing logs
		for rows.Next() {

			var callDate, phoneN, status interface{}

			if err = rows.Scan(&callDate, &phoneN, &status); err != nil && err != sql.ErrNoRows {
				return err
			}

			callLogs = append(callLogs, map[string]interface{}{
				"call_date":    callDate,
				"phone_number": bytes.NewBuffer(phoneN.([]byte)).String(),
				"call_type":    "outbound",
				"status":       status,
			})

		}

		//Get from inbound calls
		rows1, err := tx.Raw(`
			SELECT
				call_date,
				phone_number,
				status
			FROM vicidial_closer_log
			WHERE
				user=? and call_date >= ?  and call_date <=? order by call_date desc limit 10000
		`, params.Username, BeginDate, EndDate).Rows()

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		for rows1.Next() {

			var callDate, phoneN, status interface{}

			if err = rows1.Scan(&callDate, &phoneN, &status); err != nil && err != sql.ErrNoRows {
				return err
			}

			callLogs = append(callLogs, map[string]interface{}{
				"call_date":    callDate,
				"phone_number": bytes.NewBuffer(phoneN.([]byte)).String(),
				"call_type":    "inbound",
				"status":       status,
			})
		}

		return nil
	}()

	// 11. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[FETCH LOGS] %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	// 12. success
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusText(http.StatusOK),
		"logs":   callLogs,
	})
	c.Abort()
	return
}
