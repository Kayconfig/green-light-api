package main

import (
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kayconfig/green-light-api/internal/data"
	"github.com/kayconfig/green-light-api/internal/validator"
	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	// define a client struct to hold the rate limiter and last seen time for each
	// client
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// launch a background goroutine which removes old entries from the clients map once
	// every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			// Lock the mutex to prevent any rate limiter checks from happening while
			// the cleanup is taking place
			mu.Lock()

			// loop through all clients. If they haven't been seen within the last
			// three minutes, delete the corresponding entry from the map
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			// important to unlock mutex when cleanup is complete
			mu.Unlock()
		}
	}()

	// the function we are returning is a closure, which 'closes over' the limiter
	// variable
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// only carry out the check if rate limiting is enabled
		if app.config.limiter.enabled {
			ip := realip.FromRequest(r)

			// lock mutex to prevent code from being executed  concurrently
			mu.Lock()

			if _, found := clients[ip]; !found {
				clients[ip] = &client{limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst)}
			}

			// Update the last seen time for the client.
			clients[ip].lastSeen = time.Now()

			// Call limiter.Allow() to see if the request is permitted, and if it's not,
			// then we call the rateLimitedExceededREsponse() helper to return a 429 Too many
			// Requests response
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}

			// unlock the mutex before call the next handler in the chain
			// We don't use defer to unlock the mutex, so that we won't need
			// to wait till all the handlers downstream returns before unlocking the mutex
			mu.Unlock()
		}

		next.ServeHTTP(w, r)

	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// add the "Vary: Authorization" header to the response. This indicates to any
		// caches that the response may vary based on the value of the Authorization
		// header in the request
		w.Header().Add("Vary", "Authorization")

		// defaults to empty string if not defined
		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		token := headerParts[1]

		v := validator.New()

		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}

			return
		}

		r = app.contextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) requireActivatedUser(next http.Handler) http.Handler {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
	return app.requireAuthenticatedUser(fn)
}

func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		userPermissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		if !slices.Contains(userPermissions, code) {
			app.notPermittedResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

	return app.requireActivatedUser(fn).ServeHTTP
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "origin")
		w.Header().Set("Vary", "Access-Control-Request-Method")

		origin := w.Header().Get("origin")
		if origin != "" {
			for i := range app.config.cors.trustedOrigins {
				if app.config.cors.trustedOrigins[i] == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)

					// determine if the request is pre-flight
					if r.Method == "OPTIONS" && w.Header().Get("Access-Control-Request-Method") != "" {
						w.Header().Set("Acess-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
						w.WriteHeader(http.StatusOK)
						return
					}
					break
				}

			}
		}
		next.ServeHTTP(w, r)
	})
}

type metricsResponseWriter struct {
	wrapped        http.ResponseWriter
	statusCode     int
	headersWritten bool
}

func NewMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		wrapped:        w,
		statusCode:     http.StatusOK,
		headersWritten: false,
	}
}

func (m *metricsResponseWriter) Header() http.Header {
	return m.wrapped.Header()
}

func (m *metricsResponseWriter) WriteHeader(statusCode int) {
	m.wrapped.WriteHeader(statusCode)
	if !m.headersWritten {
		m.headersWritten = true
		m.statusCode = statusCode
	}
}
func (m *metricsResponseWriter) Write(b []byte) (int, error) {
	m.headersWritten = true
	return m.wrapped.Write(b)
}

func (m *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return m.wrapped
}

func (app *application) metrics(next http.Handler) http.Handler {
	var (
		totalRequestReceived            = expvar.NewInt("total_requests_received")
		totalResponsesSent              = expvar.NewInt("total_responses_sent")
		totalProcessingTimeMilliseconds = expvar.NewInt("total_processing_time_μs")
		totalResponseSentByStatus       = expvar.NewMap("total_repsonses_sent_by_status")
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		totalRequestReceived.Add(1)

		mw := NewMetricsResponseWriter(w)

		next.ServeHTTP(mw, r)

		totalResponsesSent.Add(1)

		totalResponseSentByStatus.Add(strconv.Itoa(mw.statusCode), 1)

		totalProcessingTimeMilliseconds.Add(int64(time.Since(start).Microseconds()))
	})
}
