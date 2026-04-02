package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/kayconfig/green-light-api/internal/data"
)

func TestHealthcheck(t *testing.T) {
	mustRequireDB(t)
	resetDB(t)

	resp := makeRequest(t, http.MethodGet, "/v1/healthcheck", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := mustDecodeBody(t, resp)
	if body["status"] != "available" {
		t.Errorf("got status %q, want %q", body["status"], "available")
	}
}

func TestRegisterUser(t *testing.T) {
	mustRequireDB(t)

	tests := []struct {
		name        string
		setup       func(t *testing.T)
		payload     any
		wantStatus  int
		checkMailer bool
	}{
		{
			name:        "valid registration",
			payload:     map[string]any{"name": "Alice", "email": "alice@example.com", "password": "password123"},
			wantStatus:  http.StatusAccepted,
			checkMailer: true,
		},
		{
			name:       "missing name",
			payload:    map[string]any{"email": "bob@example.com", "password": "password123"},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid email",
			payload:    map[string]any{"name": "Carol", "email": "not-an-email", "password": "password123"},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "password too short",
			payload:    map[string]any{"name": "Dave", "email": "dave@example.com", "password": "short"},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "duplicate email",
			setup: func(t *testing.T) {
				mustCreateUser(t, "Eve", "eve@example.com", "password123")
			},
			payload:    map[string]any{"name": "Eve Two", "email": "eve@example.com", "password": "password123"},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "malformed JSON",
			payload:    "this is not json",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			if tc.setup != nil {
				tc.setup(t)
			}

			resp := makeRequest(t, http.MethodPost, "/v1/users", tc.payload, "")
			if resp.StatusCode != tc.wantStatus {
				body := mustDecodeBody(t, resp)
				t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, tc.wantStatus, body)
			} else {
				resp.Body.Close()
			}

			if tc.checkMailer {
				testApp.wg.Wait()
				if testMailer.count() == 0 {
					t.Error("expected mailer to be called, but no messages were sent")
				} else {
					msg := testMailer.last()
					if msg.Template != "user_welcome.tmpl" {
						t.Errorf("got mailer template %q, want %q", msg.Template, "user_welcome.tmpl")
					}
				}
			}
		})
	}
}

func TestActivateUser(t *testing.T) {
	mustRequireDB(t)

	t.Run("valid activation token", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
		token, err := testApp.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
		if err != nil {
			t.Fatalf("create activation token: %v", err)
		}

		resp := makeRequest(t, http.MethodPut, "/v1/users/activated",
			map[string]any{"token": token.Plaintext}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body := mustDecodeBody(t, resp)
			t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, http.StatusOK, body)
		}
	})

	t.Run("token too short", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodPut, "/v1/users/activated",
			map[string]any{"token": "short"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("valid format but nonexistent token", func(t *testing.T) {
		resetDB(t)
		// 26-char string that will pass format validation but won't exist in DB
		resp := makeRequest(t, http.MethodPut, "/v1/users/activated",
			map[string]any{"token": "AAAABBBBCCCCDDDDEEEEFFFFGG"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})
}

func TestCreateAuthToken(t *testing.T) {
	mustRequireDB(t)

	tests := []struct {
		name       string
		setup      func(t *testing.T) (email, password string)
		wantStatus int
	}{
		{
			name: "valid credentials for activated user",
			setup: func(t *testing.T) (string, string) {
				user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
				mustActivateUser(t, user)
				return "alice@example.com", "password123"
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "wrong password",
			setup: func(t *testing.T) (string, string) {
				user := mustCreateUser(t, "Bob", "bob@example.com", "password123")
				mustActivateUser(t, user)
				return "bob@example.com", "wrongpassword"
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "nonexistent email",
			setup: func(t *testing.T) (string, string) {
				return "nobody@example.com", "password123"
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			email, password := tc.setup(t)

			resp := makeRequest(t, http.MethodPost, "/v1/tokens/authentication",
				map[string]any{"email": email, "password": password}, "")
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body := mustDecodeBody(t, resp)
				t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, tc.wantStatus, body)
			}
		})
	}

	t.Run("missing fields", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodPost, "/v1/tokens/authentication",
			map[string]any{}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})
}

func TestSendActivationToken(t *testing.T) {
	mustRequireDB(t)

	t.Run("existing unactivated user", func(t *testing.T) {
		resetDB(t)
		mustCreateUser(t, "Alice", "alice@example.com", "password123")

		resp := makeRequest(t, http.MethodPost, "/v1/users/verification",
			map[string]any{"email": "alice@example.com"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		testApp.wg.Wait()
		if testMailer.count() == 0 {
			t.Error("expected mailer to be called for unactivated user, but no messages were sent")
		}
	})

	t.Run("unknown email returns 200 (anti-enumeration)", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodPost, "/v1/users/verification",
			map[string]any{"email": "nobody@example.com"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("invalid email format", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodPost, "/v1/users/verification",
			map[string]any{"email": "not-an-email"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})
}

func TestPasswordResetFlow(t *testing.T) {
	mustRequireDB(t)

	t.Run("request password reset - activated user", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
		mustActivateUser(t, user)

		resp := makeRequest(t, http.MethodPost, "/v1/tokens/password-reset",
			map[string]any{"email": "alice@example.com"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusAccepted)
		}

		testApp.wg.Wait()
		if testMailer.count() == 0 {
			t.Error("expected password reset email to be sent")
		}
	})

	t.Run("request password reset - unactivated user", func(t *testing.T) {
		resetDB(t)
		mustCreateUser(t, "Bob", "bob@example.com", "password123")

		resp := makeRequest(t, http.MethodPost, "/v1/tokens/password-reset",
			map[string]any{"email": "bob@example.com"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("request password reset - unknown email", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodPost, "/v1/tokens/password-reset",
			map[string]any{"email": "nobody@example.com"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("update password with valid reset token", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Carol", "carol@example.com", "password123")
		mustActivateUser(t, user)
		token, err := testApp.models.Tokens.New(user.ID, 15*time.Minute, data.ScopePasswordReset)
		if err != nil {
			t.Fatalf("create reset token: %v", err)
		}

		resp := makeRequest(t, http.MethodPut, "/v1/users/password",
			map[string]any{"password_reset_token": token.Plaintext, "new_password": "newpassword123"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body := mustDecodeBody(t, resp)
			t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, http.StatusOK, body)
		}
	})

	t.Run("update password with invalid token", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodPut, "/v1/users/password",
			map[string]any{"password_reset_token": "AAAABBBBCCCCDDDDEEEEFFFFGG", "new_password": "newpassword123"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("update password too short", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Dave", "dave@example.com", "password123")
		mustActivateUser(t, user)
		token, err := testApp.models.Tokens.New(user.ID, 15*time.Minute, data.ScopePasswordReset)
		if err != nil {
			t.Fatalf("create reset token: %v", err)
		}

		resp := makeRequest(t, http.MethodPut, "/v1/users/password",
			map[string]any{"password_reset_token": token.Plaintext, "new_password": "short"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})
}

func TestShowMovie(t *testing.T) {
	mustRequireDB(t)

	tests := []struct {
		name       string
		path       func(movieID int64) string
		token      func(t *testing.T) string
		wantStatus int
	}{
		{
			name: "valid ID with movies:read permission",
			path: func(movieID int64) string { return fmt.Sprintf("/v1/movies/%d", movieID) },
			token: func(t *testing.T) string {
				user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
				mustActivateUser(t, user)
				return mustCreateAuthToken(t, user.ID)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "movie not found",
			path: func(_ int64) string { return "/v1/movies/999999" },
			token: func(t *testing.T) string {
				user := mustCreateUser(t, "Bob", "bob@example.com", "password123")
				mustActivateUser(t, user)
				return mustCreateAuthToken(t, user.ID)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "invalid ID",
			path: func(_ int64) string { return "/v1/movies/abc" },
			token: func(t *testing.T) string {
				user := mustCreateUser(t, "Carol", "carol@example.com", "password123")
				mustActivateUser(t, user)
				return mustCreateAuthToken(t, user.ID)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "no auth token",
			path:       func(movieID int64) string { return fmt.Sprintf("/v1/movies/%d", movieID) },
			token:      func(t *testing.T) string { return "" },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "unactivated user",
			path: func(movieID int64) string { return fmt.Sprintf("/v1/movies/%d", movieID) },
			token: func(t *testing.T) string {
				user := mustCreateUser(t, "Dave", "dave@example.com", "password123")
				return mustCreateAuthToken(t, user.ID)
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			movie := mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi", "thriller"})
			token := tc.token(t)

			resp := makeRequest(t, http.MethodGet, tc.path(movie.ID), nil, token)
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body := mustDecodeBody(t, resp)
				t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, tc.wantStatus, body)
			}
		})
	}
}

func TestListMovies(t *testing.T) {
	mustRequireDB(t)

	t.Run("no movies returns empty array", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
		mustActivateUser(t, user)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodGet, "/v1/movies", nil, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("returns movies with metadata", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Bob", "bob@example.com", "password123")
		mustActivateUser(t, user)
		token := mustCreateAuthToken(t, user.ID)

		mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi"})
		mustCreateMovie(t, "Interstellar", 2014, 169, []string{"sci-fi", "drama"})

		resp := makeRequest(t, http.MethodGet, "/v1/movies", nil, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body := mustDecodeBody(t, resp)
		if body["metadata"] == nil {
			t.Error("expected metadata in response")
		}
	})

	t.Run("invalid page parameter", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Carol", "carol@example.com", "password123")
		mustActivateUser(t, user)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodGet, "/v1/movies?page=-1", nil, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("no auth token", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodGet, "/v1/movies", nil, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})
}

func TestCreateMovie(t *testing.T) {
	mustRequireDB(t)

	validPayload := map[string]any{
		"title":   "Inception",
		"year":    2010,
		"runtime": "148 mins",
		"genres":  []string{"sci-fi", "thriller"},
	}

	t.Run("valid payload with movies:write permission", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
		mustActivateUser(t, user)
		mustGrantWritePermission(t, user.ID)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodPost, "/v1/movies", validPayload, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body := mustDecodeBody(t, resp)
			t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, http.StatusCreated, body)
		}
		if resp.Header.Get("Location") == "" {
			t.Error("expected Location header to be set")
		}
	})

	t.Run("movies:read only user gets 403", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Bob", "bob@example.com", "password123")
		mustActivateUser(t, user)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodPost, "/v1/movies", validPayload, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("missing title", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Carol", "carol@example.com", "password123")
		mustActivateUser(t, user)
		mustGrantWritePermission(t, user.ID)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodPost, "/v1/movies",
			map[string]any{"year": 2010, "runtime": "148 mins", "genres": []string{"sci-fi"}}, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("year before 1888", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Dave", "dave@example.com", "password123")
		mustActivateUser(t, user)
		mustGrantWritePermission(t, user.ID)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodPost, "/v1/movies",
			map[string]any{"title": "Old Movie", "year": 1800, "runtime": "90 mins", "genres": []string{"drama"}}, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("no auth token", func(t *testing.T) {
		resetDB(t)
		resp := makeRequest(t, http.MethodPost, "/v1/movies", validPayload, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})
}

func TestUpdateMovie(t *testing.T) {
	mustRequireDB(t)

	t.Run("partial update succeeds", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
		mustActivateUser(t, user)
		mustGrantWritePermission(t, user.ID)
		token := mustCreateAuthToken(t, user.ID)
		movie := mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi"})

		resp := makeRequest(t, http.MethodPatch, fmt.Sprintf("/v1/movies/%d", movie.ID),
			map[string]any{"title": "Inception Updated"}, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body := mustDecodeBody(t, resp)
			t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, http.StatusOK, body)
		}
	})

	t.Run("movie not found", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Bob", "bob@example.com", "password123")
		mustActivateUser(t, user)
		mustGrantWritePermission(t, user.ID)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodPatch, "/v1/movies/999999",
			map[string]any{"title": "Ghost Movie"}, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("no auth token", func(t *testing.T) {
		resetDB(t)
		movie := mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi"})

		resp := makeRequest(t, http.MethodPatch, fmt.Sprintf("/v1/movies/%d", movie.ID),
			map[string]any{"title": "No Auth"}, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("read-only user gets 403", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Carol", "carol@example.com", "password123")
		mustActivateUser(t, user)
		token := mustCreateAuthToken(t, user.ID)
		movie := mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi"})

		resp := makeRequest(t, http.MethodPatch, fmt.Sprintf("/v1/movies/%d", movie.ID),
			map[string]any{"title": "Read-only User"}, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})
}

func TestDeleteMovie(t *testing.T) {
	mustRequireDB(t)

	t.Run("existing movie is deleted", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Alice", "alice@example.com", "password123")
		mustActivateUser(t, user)
		mustGrantWritePermission(t, user.ID)
		token := mustCreateAuthToken(t, user.ID)
		movie := mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi"})

		resp := makeRequest(t, http.MethodDelete, fmt.Sprintf("/v1/movies/%d", movie.ID), nil, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body := mustDecodeBody(t, resp)
			t.Errorf("got status %d, want %d; body: %v", resp.StatusCode, http.StatusOK, body)
		}
	})

	t.Run("movie not found", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Bob", "bob@example.com", "password123")
		mustActivateUser(t, user)
		mustGrantWritePermission(t, user.ID)
		token := mustCreateAuthToken(t, user.ID)

		resp := makeRequest(t, http.MethodDelete, "/v1/movies/999999", nil, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("no auth token", func(t *testing.T) {
		resetDB(t)
		movie := mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi"})

		resp := makeRequest(t, http.MethodDelete, fmt.Sprintf("/v1/movies/%d", movie.ID), nil, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("read-only user gets 403", func(t *testing.T) {
		resetDB(t)
		user := mustCreateUser(t, "Carol", "carol@example.com", "password123")
		mustActivateUser(t, user)
		token := mustCreateAuthToken(t, user.ID)
		movie := mustCreateMovie(t, "Inception", 2010, 148, []string{"sci-fi"})

		resp := makeRequest(t, http.MethodDelete, fmt.Sprintf("/v1/movies/%d", movie.ID), nil, token)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})
}

// TestFullAuthFlow exercises the complete user lifecycle end-to-end through HTTP.
func TestFullAuthFlow(t *testing.T) {
	mustRequireDB(t)
	resetDB(t)

	// Step 1: Register a new user via HTTP
	registerPayload := map[string]any{
		"name":     "Integration User",
		"email":    "integration@example.com",
		"password": "securepassword",
	}
	resp := makeRequest(t, http.MethodPost, "/v1/users", registerPayload, "")
	if resp.StatusCode != http.StatusAccepted {
		body := mustDecodeBody(t, resp)
		t.Fatalf("register: got status %d; body: %v", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Wait for background goroutine to send the activation email
	testApp.wg.Wait()

	if testMailer.count() == 0 {
		t.Fatal("expected activation email to be sent after registration")
	}
	lastMsg := testMailer.last()
	mailData, ok := lastMsg.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected mail data to be map[string]any, got %T", lastMsg.Data)
	}
	activationToken, ok := mailData["activationToken"].(string)
	if !ok || activationToken == "" {
		t.Fatal("expected activationToken in mail data")
	}

	// Step 2: Activate the user account
	resp = makeRequest(t, http.MethodPut, "/v1/users/activated",
		map[string]any{"token": activationToken}, "")
	if resp.StatusCode != http.StatusOK {
		body := mustDecodeBody(t, resp)
		t.Fatalf("activate: got status %d; body: %v", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 3: Get authentication token
	resp = makeRequest(t, http.MethodPost, "/v1/tokens/authentication",
		map[string]any{"email": "integration@example.com", "password": "securepassword"}, "")
	if resp.StatusCode != http.StatusCreated {
		body := mustDecodeBody(t, resp)
		t.Fatalf("create auth token: got status %d; body: %v", resp.StatusCode, body)
	}
	tokenBody := mustDecodeBody(t, resp)
	authTokenMap, ok := tokenBody["authentication_token"].(map[string]any)
	if !ok {
		t.Fatalf("expected authentication_token in response, got: %v", tokenBody)
	}
	bearerToken, ok := authTokenMap["token"].(string)
	if !ok || bearerToken == "" {
		t.Fatal("expected token string in authentication_token")
	}

	// Step 4: List movies (proves activated + movies:read works)
	resp = makeRequest(t, http.MethodGet, "/v1/movies", nil, bearerToken)
	if resp.StatusCode != http.StatusOK {
		body := mustDecodeBody(t, resp)
		t.Fatalf("list movies: got status %d; body: %v", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 5: Create movie fails without movies:write
	moviePayload := map[string]any{
		"title":   "Test Movie",
		"year":    2023,
		"runtime": "120 mins",
		"genres":  []string{"drama"},
	}
	resp = makeRequest(t, http.MethodPost, "/v1/movies", moviePayload, bearerToken)
	if resp.StatusCode != http.StatusForbidden {
		body := mustDecodeBody(t, resp)
		t.Fatalf("create movie without write perm: got status %d; body: %v", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 6: Grant movies:write permission directly via data layer
	user, err := testApp.models.Users.GetByEmail("integration@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	mustGrantWritePermission(t, user.ID)

	// Step 7: Create movie succeeds with movies:write
	resp = makeRequest(t, http.MethodPost, "/v1/movies", moviePayload, bearerToken)
	if resp.StatusCode != http.StatusCreated {
		body := mustDecodeBody(t, resp)
		t.Fatalf("create movie with write perm: got status %d; body: %v", resp.StatusCode, body)
	}
	createBody := mustDecodeBody(t, resp)
	movieMap, ok := createBody["movie"].(map[string]any)
	if !ok {
		t.Fatalf("expected movie in response, got: %v", createBody)
	}
	movieID := int64(movieMap["id"].(float64))

	// Step 8: Update the movie
	resp = makeRequest(t, http.MethodPatch, fmt.Sprintf("/v1/movies/%d", movieID),
		map[string]any{"title": "Updated Test Movie"}, bearerToken)
	if resp.StatusCode != http.StatusOK {
		body := mustDecodeBody(t, resp)
		t.Fatalf("update movie: got status %d; body: %v", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 9: Delete the movie
	resp = makeRequest(t, http.MethodDelete, fmt.Sprintf("/v1/movies/%d", movieID), nil, bearerToken)
	if resp.StatusCode != http.StatusOK {
		body := mustDecodeBody(t, resp)
		t.Fatalf("delete movie: got status %d; body: %v", resp.StatusCode, body)
	}
	resp.Body.Close()
}
