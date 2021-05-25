package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"smartdial/config"
	logger "smartdial/log"
	"smartdial/routes"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:     "Server",
	Aliases: []string{"serve", "s"},
	Short:   "Start API Server",
	Long:    `Start Smartdial's API Server`,
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

// logger and configuration
var (
	conf = config.GetConfig()
	log  = logger.GetLogger()
)

func run() {

	// logout idle users
	go func() {
		for {
			logoutIdleUsers()
			time.Sleep(30 * time.Minute)
		}
	}()

	//attach routes
	router := routes.Router()

	// HTTP Server
	server := &http.Server{
		Addr:           ":" + conf.GetString("app.port"),
		Handler:        router,
		ReadTimeout:    15 * time.Minute,
		WriteTimeout:   15 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	// Handle graceful shutdown on SIGINT
	idleConnectionsClosed := make(chan struct{})

	go func() {

		s := make(chan os.Signal, 1)
		signal.Notify(s, os.Interrupt, syscall.SIGTERM)
		<-s

		// We received an interrupt signal, shut down.
		if err := server.Shutdown(context.Background()); err != nil {
			log.Errorf("HTTP server shutdown error: %v\n", err)
		}

		close(idleConnectionsClosed)
	}()

	log.Infof("Starting server on http://%s\n", server.Addr)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Error(err)
		panic(err)
	}

	<-idleConnectionsClosed
}
