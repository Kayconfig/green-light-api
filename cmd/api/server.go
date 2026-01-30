package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	shutdownErrorChan := make(chan error)

	// start a background goroutine.
	go func() {
		// create a quit channel which carries os.Signal values.
		quit := make(chan os.Signal, 1)
		// use signal.Notify() to listen for incoming SIGINT and SIGTERM signals and
		// relay them to the quit channel. Any other signals will not be caught by
		// signal.Notify() and will retain their default behavior.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Read the signal from the quit channel. This code will block until a signal is
		// received
		s := <-quit

		app.logger.Info("shutting down server", "signal", s.String())

		// create a context with a 30-second timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Call Shutdown() on the server like before, but nowwe only send on the
		// shutdownError channel if it returns an error
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownErrorChan <- err
		}
		app.logger.Info("completing background tasks", "addr", srv.Addr)

		app.wg.Wait()
		shutdownErrorChan <- nil
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	// Calling Shutdown() on our server will cause ListenAndServe() to immediately
	// return a http.ErrServerClosed error. So if we see this error, it is actually a
	// good thing and an indication that the graceful shutdown has started. So we check
	// specifically for this, only returning the error if it is NOT http.ErrServerClosed
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// otherwise, we wait to receive the return value from Shutdown() on the
	// shutdonError channel. If return value is an error, we know that there was a
	// problem with the graceful shutdown and we return the error.
	err = <-shutdownErrorChan
	if err != nil {
		return err
	}

	// At this point we know that graceful shutdown completed successfully and we
	// log a "stopped server" message.
	app.logger.Info("stopped server", "addr", srv.Addr)

	return nil
}
