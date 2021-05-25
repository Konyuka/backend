package auth

import (
	"errors"
	"fmt"
	"math"
	"smartdial/models"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Auth struct
type Auth struct {
	Logger *logrus.Logger
	DB     *gorm.DB
	Redis  *redis.Client
	mu     sync.Mutex
}

// SignIn - authorize
func (a *Auth) SignIn(username, password string) (string, error) {

	var (
		err  error
		user = new(models.User)
	)

	// 1. Fetch user
	gor := a.DB.Raw(`
		SELECT pass, failed_login_count, last_login_date 
		FROM vicidial_users 
		WHERE 
			user = ? AND  
			user_level > 0 AND  
			active = 'Y';
	`, username).Scan(user)

	// 2. Throw maximum attempts exceeded
	if user.FailedLoginCount >= 10 && time.Now().Sub(user.LastLoginDate).Minutes() < 15 {

		nextLogin := math.Round(user.LastLoginDate.Add(15 * time.Minute).Sub(time.Now()).Minutes())

		return "", fmt.Errorf("maximum login attempts exceeded. try in %v minutes", nextLogin)
	}

	// 3. user non-existent or wrong credentials
	if err = gor.Error; err != nil || gor.RowsAffected == 0 || user.Pass != password {

		a.Logger.Errorf("cannot fetch user for auth. user : %s pass : %v. error : %s\n", username, password, err)

		return "", errors.New("wrong username or password")
	}

	return CreateToken(username)
}
