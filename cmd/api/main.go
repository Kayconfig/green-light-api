package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/kayconfig/green-light-api/internal/data"
	"github.com/kayconfig/green-light-api/internal/mailer"
	"github.com/kayconfig/green-light-api/migrations"
	_ "github.com/lib/pq"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config *config
	logger *slog.Logger
	models data.Models
	mailer *mailer.Mailer
	wg     sync.WaitGroup
}

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

	flag.Parse()

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

	mailer, err := mailer.New(
		cfg.smtp.host,
		cfg.smtp.port,
		cfg.smtp.username,
		cfg.smtp.password,
		cfg.smtp.sender,
	)
	if err != nil {
		logErrAndExit(err)
	}

	app := &application{
		config: &cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer,
	}

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

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}
	// set the maximum number of open (in-use + idle) connections in the ppol.
	// a value less than or equal to zero will mean there is no limit
	db.SetMaxOpenConns(cfg.db.maxOpenConns)

	//Set the max number of idle connections in the pool.
	// value less than or equal to zero mean there is no limit
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	// set the maximum idle timeout for connections in the pool.
	// duration less than or equal to zero will mean that the
	// connectinos are not closed due to their idle time
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)

	delayInSeconds := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), delayInSeconds)
	defer cancel()

	//use pingContext to establish new connection to db,
	//passing in the context we created above. if connection
	// couldn't be established within the specified delayInSeconds
	// the following will return an error
	// if we get error, we close the connection pool and return error
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
