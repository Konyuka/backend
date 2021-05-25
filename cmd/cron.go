package cmd

import (
	"fmt"
	"smartdial/controllers/auth"
	"smartdial/data"
	"smartdial/jobs"
	"strings"

	l "log"

	"github.com/jasonlvhit/gocron"
	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:     "cron",
	Aliases: []string{"jobs", "c"},
	Short:   "Run background tasks e.g Cleaning up records in tables",
	Run: func(cmd *cobra.Command, args []string) {
		runTasks()
	},
}

func init() {
	rootCmd.AddCommand(cronCmd)
}

// do the thing
func runTasks() {

	l.Printf("Starting background jobs....\n")

	var cjob = jobs.NewCronJob()

	if err := gocron.Every(5).Second().Do(cjob.LogoutInactive); err != nil {
		log.Errorf("cannot prepare logout inactivity job : %v", err)
	}

	//run scheduler
	<-gocron.Start()
}

// logoutIdleUsers -
func logoutIdleUsers() {

	var (
		db  = data.GetDB()
		err error
	)

	err = func() error {

		rows, err := db.Raw(`
			SELECT extension FROM vicidial_live_agents
			WHERE
				random_id = '10' OR (last_update_time + INTERVAL 30 Minute) < NOW()
		`).Rows()

		if err != nil {
			return fmt.Errorf("cannot select extension from live agent. %v", err)
		}

		// logout all extensions
		for rows.Next() {

			var extension interface{}

			if err = rows.Scan(&extension); err != nil {
				return err
			}

			if extension != nil {

				// log out phone
				if err = auth.LogoutPhone(db, strings.Split(fmt.Sprintf("%s", extension), "/")[1]); err != nil {
					return err
				}

				// delete record from live agent
				err = db.Exec(`DELETE FROM vicidial_live_agents WHERE extension = ?;`, extension).Error

				if err != nil {
					return err
				}
			}
		}

		return nil
	}()

	if err != nil {
		log.Errorf("[LOGOUT IDLE USERS] cannot select extension from live agent. %v", err)
	}

}
