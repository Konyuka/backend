package calls

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"smartdial/data"
	"smartdial/log"
	"smartdial/models"
)

//DeleteCallBack - DeleteCallMenu
func DeleteCallBack(c *gin.Context) {

	var (
		err    error
		params = new(models.VicidialCallback)
		db     = data.GetDB()
		tx     = db.Begin()
		Logger = log.GetLogger()
	)

	// begin a transaction
	err = func() error {

		if err = c.BindJSON(params); err != nil {
			Logger.Errorf("cannot parse delete call back request. %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"status": http.StatusText(http.StatusBadRequest),
				"error":  fmt.Sprintf("%v", err),
			})
			c.Abort()
			return err
		}

		fmt.Println("this is the step", params.CallbackID)

		err = db.Exec(`DELETE FROM  vicidial_callbacks where callback_id=?;`, params.CallbackID).Error

		if err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		tx.Rollback()
		fmt.Println("Error: ", err)
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
		"status":  http.StatusText(http.StatusOK),
		"Message": "Call Back Deleted Successfully",
	})
	c.Abort()
	return
}
