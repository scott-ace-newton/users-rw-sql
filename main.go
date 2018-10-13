package main

import (
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	"github.com/scott-ace-newton/users-rw-sql/notification"
	"github.com/scott-ace-newton/users-rw-sql/persistence"
	"github.com/scott-ace-newton/users-rw-sql/users"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	appName = "users-rw-sql"
	appDescription = "Application for creating, updating, returning and deleting users from a sql db"
)

func main() {
	app := cli.App(appName, appDescription)

	sqlCredentials := app.String(cli.StringOpt{
		Name:      "sqlCredentials",
		Desc:      "Username and password to connect to db, should be in 'user:pass' format",
		EnvVar:    "SQL_CREDENTIALS",
		HideValue: true,
	})
	sqlDSN := app.String(cli.StringOpt{
		Name:      "sqlDSN",
		Desc:      "DSN to connect to the DB e.g. user:pass@host/schema",
		EnvVar:    "SQL_DSN",
		HideValue: true,
	})
	queueURL := app.String(cli.StringOpt{
		Name:      "carouselQueueURL",
		Desc:      "Url of queue to send messages to",
		EnvVar:    "QUEUE_URL",
		HideValue: true,
	})
	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})
	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  "info",
		Desc:   "App log level",
		EnvVar: "LOG_LEVEL",
	})

	logLvl, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.WithField("logLevel", logLevel).WithError(err).Error("could not parse log level. Using INFO instead.")
		logLvl = log.InfoLevel
	}
	log.SetLevel(logLvl)
	log.Infof("[Startup] %s is starting on port %s...", appName, port)

	app.Action = func() {
		if *queueURL == "" {
			log.Fatal("queue url not set")
			return
		}
		if *sqlDSN == "" {
			log.Fatal("SQL connection string not set")
			return
		}
		if *sqlCredentials == "" {
			log.Fatalf("SQL Username and password not set")
			return
		}

		sqlClient, err := persistence.NewClient(*sqlDSN, *sqlCredentials)
		if err != nil {
			return
		}

		queueClient := notification.NewQueueClient(*queueURL)

		h := users.NewUsersHandler(sqlClient, queueClient)
		r := mux.NewRouter()
		h.RegisterHandlers(r)

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

		go func() {
			log.Infof("Listening on port %v", *port)
			if err := http.ListenAndServe("/", r); err != nil {
				log.Errorf("HTTP server got shut down error: %v", err)
			}
			sig <- os.Interrupt
		}()

		<-sig
		log.Info("shutting down HTTP server...")
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}
}
