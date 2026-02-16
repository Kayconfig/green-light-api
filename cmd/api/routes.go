package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kayconfig/green-light-api/internal/data"
)

func (app *application) routes() http.Handler {
	router := chi.NewRouter()

	router.Get("/v1/healthcheck", app.healthCheckHandler)

	// movies
	router.Group(func(movieRouter chi.Router) {
		movieRouter.Use(app.requireActivatedUser)

		movieRouter.Get("/v1/movies/{id}", app.requirePermission(data.PermissionsCode.MoviesRead, app.showMovieHandler))
		movieRouter.Get("/v1/movies", app.requirePermission(data.PermissionsCode.MoviesRead, app.listMoviesHandler))
		movieRouter.Post("/v1/movies", app.requirePermission(data.PermissionsCode.MoviesWrite, app.createMovieHandler))
		movieRouter.Patch("/v1/movies/{id}", app.requirePermission(data.PermissionsCode.MoviesWrite, app.updateMovieHandler))
		movieRouter.Delete("/v1/movies/{id}", app.requirePermission(data.PermissionsCode.MoviesWrite, app.deleteMovieHandler))
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
		app.enableCORS(
			app.rateLimit(
				app.authenticate(router),
			),
		))

}
