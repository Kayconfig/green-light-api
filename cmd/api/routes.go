package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) routes() http.Handler {
	router := chi.NewRouter()

	router.Get("/v1/healthcheck", app.healthCheckHandler)

	// movies
	router.Group(func(movieRouter chi.Router) {
		movieRouter.Use(app.requireActivatedUser)
		movieRouter.Get("/v1/movies/{id}", app.showMovieHandler)
		movieRouter.Post("/v1/movies", app.createMovieHandler)
		movieRouter.Patch("/v1/movies/{id}", app.updateMovieHandler)
		movieRouter.Delete("/v1/movies/{id}", app.deleteMovieHandler)
		movieRouter.Get("/v1/movies", app.listMoviesHandler)
	})

	// users
	router.Post("/v1/users", app.registerUserHandler)
	router.Post("/v1/users/verification", app.sendActivationTokenHandler)
	router.Put("/v1/users/activated", app.activateUserHandler)

	//authentication
	router.Post("/v1/tokens/authentication", app.createAuthenticationTokenHandler)

	router.NotFound(app.notFoundResponse)
	router.MethodNotAllowed(app.methodNotAllowedResponse)

	return app.recoverPanic(
		app.rateLimit(
			app.authenticate(router),
		),
	)

}
