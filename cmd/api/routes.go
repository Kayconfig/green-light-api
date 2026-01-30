package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) routes() http.Handler {
	router := chi.NewRouter()

	router.Get("/v1/healthcheck", app.healthCheckHandler)
	// movies
	router.Get("/v1/movies/{id}", app.showMovieHandler)
	router.Post("/v1/movies", app.createMovieHandler)
	router.Patch("/v1/movies/{id}", app.updateMovieHandler)
	router.Delete("/v1/movies/{id}", app.deleteMovieHandler)
	router.Get("/v1/movies", app.listMoviesHandler)

	// users
	router.Post("/v1/users", app.registerUserHandler)
	router.Post("/v1/users/verification", app.sendActivationTokenHandler)
	router.Put("/v1/users/activated", app.activateUserHandler)

	router.NotFound(app.notFoundResponse)
	router.MethodNotAllowed(app.methodNotAllowedResponse)

	return app.recoverPanic(app.rateLimit(router))

}
