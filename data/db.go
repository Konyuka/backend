package data

import (
	"smartdial/config"
	"smartdial/log"
	"time"

	_ "github.com/go-sql-driver/mysql" // mysql dialect
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" //psql dialect
	_ "github.com/lib/pq"
)

var db *gorm.DB

var (
	l   = log.GetLogger()
	err error
	// ENV -
	ENV string
)

// create a connection to mysql database and
func connect() error {

	// recover from panic
	defer func() {
		if r := recover(); r != nil {
			l.Infof("connection with database recovered")
		}
	}()

	cg := config.GetConfig()

	// Set environment
	ENV = cg.GetString("app.environment")

	dbDriver := cg.GetString("database.driver")
	dbHost := cg.GetString("database.host")
	dbPort := cg.GetString("database.port")
	dbName := cg.GetString("database.dbname")
	dbUser := cg.GetString("database.user")
	dbPass := cg.GetString("database.pass")

	db, err = gorm.Open(dbDriver,
		dbUser+":"+dbPass+"@tcp("+dbHost+":"+dbPort+")/"+dbName+"?charset=utf8&parseTime=True&loc=Local&allowNativePasswords=true")

	db.DB().SetMaxOpenConns(10)
	db.DB().SetMaxIdleConns(20)
	db.DB().SetConnMaxLifetime(5 * time.Minute)

	if err != nil {
		l.Panicf("cannot connect to db : %v", err)
	}

	// If we're in production mode, set Gin to "release" mode
	if ENV != "production" {
		db.LogMode(true)
	}

	return db.DB().Ping()
}

//GetDB ...
func GetDB() *gorm.DB {

	for range make([]int, 10) {

		if err = connect(); err == nil {
			return db
		}

		time.Sleep(2 * time.Second)
	}

	return nil
}
