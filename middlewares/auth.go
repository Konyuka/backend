package middleware

import (
	"net/http"
	"smartdial/controllers/auth"

	"github.com/gin-gonic/gin"
)

// Auth -
func Auth() gin.HandlerFunc {

	return func(c *gin.Context) {

		if err := auth.TokenValid(c.Request); err != nil {

			c.JSON(http.StatusUnauthorized, gin.H{
				"status": http.StatusText(http.StatusUnauthorized),
			})

			c.Abort()

			return
		}

		c.Next()
	}
}
