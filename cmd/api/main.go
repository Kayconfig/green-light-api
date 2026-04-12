package main

import (
	"expvar"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/kayconfig/green-light-api/cmd/api/docs"
	"github.com/kayconfig/green-light-api/internal/data"
	"github.com/kayconfig/green-light-api/internal/vcs"
	"github.com/kayconfig/green-light-api/migrations"
	_ "github.com/lib/pq"
)

var (
	version = vcs.Version()
)

// @title           Greenlight Restful API
// @version         1.1.0
// @description     A movie management REST API. Movie endpoints require authentication via a Bearer token obtained from POST /v1/tokens/authentication.
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Enter the token with the "Bearer " prefix, e.g. "Bearer abcde12345"
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logErrAndExit := func(err error) {
		logger.Error(err.Error())
		os.Exit(1)

	}
	err := godotenv.Load()
	if err != nil {
		logErrAndExit(err)
	}

	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GOOSE_DBSTRING"), "PostgreSQL DSN")

	// Read the connection pool settings from command-line flags into the config struct.
	// Notice that the default values we're using are the ones we discussed above?
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max  connection idle time")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	//SMTP
	flag.StringVar(&cfg.smtp.host, "smtp-host", os.Getenv("SMTP_HOST"), "SMTP host")
	SMTP_PORT, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		logErrAndExit(err)
	}
	flag.IntVar(&cfg.smtp.port, "smtp-port", SMTP_PORT, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("SMTP_SENDER"), "SMTP sender")

	// parse trusted origins
	flag.Func("cors-trusted-origins", "Trusted origins (space separated)", func(s string) error {
		cfg.cors.trustedOrigins = strings.Fields(s)
		return nil
	})

	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.StringVar(&cfg.url, "api-url", os.Getenv("api-url"), "The url for this api")

	flag.Parse()

	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	db, err := openDB(cfg)
	if err != nil {
		logErrAndExit(err)
	}
	defer db.Close()
	logger.Info("database connection pool established")

	// set version expvar
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))

	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

	mailer, err := NewMailer(cfg.smtp)
	if err != nil {
		logErrAndExit(err)
	}

	app := NewApplication(
		&cfg,
		logger,
		data.NewModels(db),
		mailer,
	)

	// run migration, if env=development
	if cfg.env == "development" {
		err := app.RunMigration(db, migrations.FS, ".")
		if err != nil {
			logErrAndExit(err)
		}
	}

	err = app.serve()
	if err != nil {
		logErrAndExit(err)
	}
}
