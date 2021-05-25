package log

import (
	"fmt"
	"os"

	"smartdial/config"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log = logrus.New()

func init() {

	c := config.GetConfig()

	// in dev
	if c.GetString("app.environment") != "production" {

		log.Formatter = &logrus.TextFormatter{
			TimestampFormat: "2020-01-02 23:12:01",
			FullTimestamp:   true,
		}

		log.Out = os.Stdout

		log.Level = logrus.DebugLevel
	}

	log.Formatter = &logrus.JSONFormatter{}

	log.SetOutput(&lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/smartdial.log", c.GetString("app.log_path")),
		MaxSize:    200, //mbs,
		MaxBackups: 2,
		MaxAge:     28, //days
	})

	log.SetLevel(logrus.InfoLevel)
}

// GetLogger - returns log
func GetLogger() *logrus.Logger {
	return log
}
