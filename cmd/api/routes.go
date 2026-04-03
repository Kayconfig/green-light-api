package main

import (
	"expvar"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kayconfig/green-light-api/internal/data"
	httpSwagger "github.com/swaggo/http-swagger/v2"
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
	router.Put("/v1/users/password", app.updatePasswordHandler)

	// authentication
	router.Post("/v1/tokens/authentication", app.createAuthenticationTokenHandler)
	router.Post("/v1/tokens/password-reset", app.passwordResetHandler)

	// metrics
	router.Get("/v1/metrics", expvar.Handler().ServeHTTP)

	router.NotFound(app.notFoundResponse)
	router.MethodNotAllowed(app.methodNotAllowedResponse)

	// Wrap all API routes with the full middleware chain (including rate limiter).
	// Swagger routes are registered on a separate top-level router so they bypass rate limiting.
	rateLimited := app.metrics((app.enableCORS(
		app.rateLimit(
			app.authenticate(router),
		),
	)))

	top := chi.NewRouter()
	top.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL(fmt.Sprintf("%s/swagger/doc.json", app.config.url)),
	))
	top.Mount("/", rateLimited)

	return app.recoverPanic(top)
}
