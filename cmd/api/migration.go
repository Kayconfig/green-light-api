package main

import (
	"database/sql"
	"io/fs"

	"github.com/pressly/goose/v3"
)

func Migrate(db *sql.DB, dir string) error {
	err := goose.SetDialect("postgres")
	if err != nil {
		return err
	}
	err = goose.Up(db, dir)
	return err
}

func (app *application) RunMigration(db *sql.DB, migrationFS fs.FS, dir string) error {
	goose.SetBaseFS(migrationFS)
	defer func() {
		goose.SetBaseFS(nil)
	}()
	app.logger.Info("running migration...")
	err := Migrate(db, dir)
	if err != nil {
		app.logger.Error(err.Error())
		return err
	}
	app.logger.Info("migration successful!")
	return nil
}
