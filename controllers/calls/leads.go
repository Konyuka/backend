package calls

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"smartdial/models"
)

// FetchLogs -
func LeadsCount(c *gin.Context) {

	var (
		params     = new(models.LeadsRequest)
		err        error
		tx         = call.DB.Begin()
		leadsCount interface{}
	)

	// 1. parse request
	if err = c.BindJSON(params); err != nil {
		call.Logger.Errorf("cannot parse fetch leads count request. %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusText(http.StatusBadRequest),
			"error":  fmt.Sprintf("%v", err),
		})
		c.Abort()
		return
	}

	err = func() error {

		err = tx.Raw(`SELECT COUNT(*) FROM vicidial_list vl INNER JOIN vicidial_lists vls ON vl.list_id = vls.list_id WHERE vls.campaign_id = ? AND vl.status = 'NEW';`, params.Campaign).Row().Scan(&leadsCount)

		if err != nil {
			return err
		}

		return nil
	}()

	// 11. handle error response
	if err != nil {
		tx.Rollback()
		call.Logger.Errorf("[FETCH LEADS COUNT] %v", err)
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
		"leads":  leadsCount,
	})
	c.Abort()
	return
}
