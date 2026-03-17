package main

import (
	"context"
	"database/sql"
	"time"
)

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
