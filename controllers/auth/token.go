package auth

import (
	"errors"
	"fmt"
	"net/http"
	"smartdial/config"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis"
)

var (
	mu sync.Mutex
	c  = config.GetConfig()
	// SECRET -
	SECRET = c.GetString("app.secret")
)

// CreateToken - create a token
func CreateToken(username string) (string, error) {

	claims := jwt.MapClaims{
		"user_id":    username,
		"exp":        time.Now().Add(12 * time.Hour).Unix(),
		"authorized": true,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(SECRET))
}

// ExtractToken -
func ExtractToken(r *http.Request) string {

	bearerToken := r.Header.Get("Authorization")

	vals := strings.Split(bearerToken, " ")

	if len(vals) == 2 {

		uid, _ := TokenID(vals[1])

		if err := CachedToken(uid, vals[1]); err != redis.Nil && err == nil {

			return vals[1]
		}

		return ""
	}

	return ""
}

// TokenValid - is it?
func TokenValid(r *http.Request) error {

	tokenString := ExtractToken(r)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected Signing method: %v", token.Header["alg"])
		}
		return []byte(SECRET), nil
	})

	if err != nil {
		return err
	}

	if _, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// p, _ := json.MarshalIndent(claims, "", " ")
		return nil
	}

	return nil
}

// TokenID - extract UserID
func TokenID(tokenString string) (string, error) {

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected Signing method: %v", token.Header["alg"])
		}
		return []byte(SECRET), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return fmt.Sprintf("%s", claims["user_id"]), nil
	}

	return "", nil
}

// CachedToken -
func CachedToken(uid, token string) error {

	var tCache interface{}

	err := auth.DB.Raw(`SELECT token FROM tokens WHERE username = ?;`, uid).Row().Scan(&tCache)

	if fmt.Sprintf("%s", tCache) != token {
		return errors.New("invalid token")
	}

	return err
}
