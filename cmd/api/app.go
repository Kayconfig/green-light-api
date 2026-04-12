package main

import (
	"log/slog"
	"sync"
	"time"

	"github.com/kayconfig/green-light-api/internal/data"
)

type smtp struct {
	host     string
	port     int
	username string
	password string
	sender   string
}
type config struct {
	url  string
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
	smtp
	cors struct {
		trustedOrigins []string
	}
}

type Mailer interface {
	Send(recipient string, templateFile string, data any) error
}

type application struct {
	config *config
	logger *slog.Logger
	models data.Models
	mailer Mailer
	wg     sync.WaitGroup
}

func NewApplication(
	config *config,
	logger *slog.Logger,
	models data.Models,
	mailer Mailer,
) *application {
	return &application{
		config,
		logger,
		models,
		mailer,
		sync.WaitGroup{},
	}
}
