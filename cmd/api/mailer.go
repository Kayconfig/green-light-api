package main

import "github.com/kayconfig/green-light-api/internal/mailer"

func NewMailer(config smtp) (*mailer.Mailer, error) {
	return mailer.New(
		config.host,
		config.port,
		config.username,
		config.password,
		config.sender,
	)
}
