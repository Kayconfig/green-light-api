package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/kayconfig/green-light-api/internal/data"
	"github.com/kayconfig/green-light-api/migrations"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

var (
	testDB     *sql.DB
	testApp    *application
	testTS     *httptest.Server
	testMailer *mockMailer
)

// mockMailer captures sent emails for test assertions.
type mockMailer struct {
	mu       sync.Mutex
	messages []mockMail
}

type mockMail struct {
	Recipient string
	Template  string
	Data      any
}

func (m *mockMailer) Send(recipient, template string, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, mockMail{recipient, template, data})
	return nil
}

func (m *mockMailer) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

func (m *mockMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockMailer) last() mockMail {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages[len(m.messages)-1]
}

// TestMain sets up a shared test server for all integration tests.
// Set TEST_DB_DSN to a PostgreSQL DSN to enable integration tests.
// Example: TEST_DB_DSN="postgres://user:pass@localhost/greenlight_test?sslmode=disable"
func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		os.Exit(m.Run())
	}

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		panic("failed to open test DB: " + err.Error())
	}
	if err = testDB.Ping(); err != nil {
		panic("failed to ping test DB: " + err.Error())
	}

	goose.SetBaseFS(migrations.FS)
	if err = goose.SetDialect("postgres"); err != nil {
		panic("goose set dialect: " + err.Error())
	}
	if err = goose.Up(testDB, "."); err != nil {
		panic("goose up: " + err.Error())
	}
	goose.SetBaseFS(nil)

	testMailer = &mockMailer{}

	cfg := &config{env: "test"}
	cfg.limiter.enabled = false

	testApp = NewApplication(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		data.NewModels(testDB),
		testMailer,
	)

	// routes() calls metrics() which calls expvar.NewInt — only safe to call once per process.
	testTS = httptest.NewServer(testApp.routes())

	code := m.Run()

	testTS.Close()
	testDB.Close()
	os.Exit(code)
}

// mustRequireDB skips the test if no DB is available.
func mustRequireDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("TEST_DB_DSN not set — skipping integration test")
	}
}

// resetDB truncates all test data between tests.
// The permissions table is intentionally excluded — its rows are seeded by migrations.
func resetDB(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(`TRUNCATE users, tokens, movies, users_permissions RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("resetDB: %v", err)
	}
	testMailer.reset()
}

// mustCreateUser inserts a user directly via the data layer and adds movies:read permission.
func mustCreateUser(t *testing.T, name, email, password string) *data.User {
	t.Helper()
	user := &data.User{
		Name:      name,
		Email:     email,
		Activated: false,
	}
	if err := user.Password.Set(password); err != nil {
		t.Fatalf("mustCreateUser: set password: %v", err)
	}
	if err := testApp.models.Users.Insert(user); err != nil {
		t.Fatalf("mustCreateUser: insert: %v", err)
	}
	if err := testApp.models.Permissions.AddForUser(user.ID, data.PermissionsCode.MoviesRead); err != nil {
		t.Fatalf("mustCreateUser: add permission: %v", err)
	}
	return user
}

// mustActivateUser sets Activated=true on the user via the data layer.
func mustActivateUser(t *testing.T, user *data.User) {
	t.Helper()
	user.Activated = true
	if err := testApp.models.Users.Update(user); err != nil {
		t.Fatalf("mustActivateUser: %v", err)
	}
}

// mustCreateAuthToken creates an authentication token for the user via the data layer
// and returns the plaintext token string.
func mustCreateAuthToken(t *testing.T, userID int64) string {
	t.Helper()
	token, err := testApp.models.Tokens.New(userID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		t.Fatalf("mustCreateAuthToken: %v", err)
	}
	return token.Plaintext
}

// mustGrantWritePermission adds movies:write permission to the user.
func mustGrantWritePermission(t *testing.T, userID int64) {
	t.Helper()
	if err := testApp.models.Permissions.AddForUser(userID, data.PermissionsCode.MoviesWrite); err != nil {
		t.Fatalf("mustGrantWritePermission: %v", err)
	}
}

// mustCreateMovie inserts a movie directly via the data layer.
func mustCreateMovie(t *testing.T, title string, year int32, runtime data.Runtime, genres []string) *data.Movie {
	t.Helper()
	movie := &data.Movie{
		Title:   title,
		Year:    year,
		Runtime: runtime,
		Genres:  genres,
	}
	if err := testApp.models.Movies.Insert(movie); err != nil {
		t.Fatalf("mustCreateMovie: %v", err)
	}
	return movie
}

// makeRequest sends an HTTP request to the test server and returns the response.
// body is JSON-encoded if non-nil. token is added as a Bearer Authorization header if non-empty.
func makeRequest(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("makeRequest: marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, testTS.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("makeRequest: new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := testTS.Client().Do(req)
	if err != nil {
		t.Fatalf("makeRequest: do: %v", err)
	}
	return resp
}

// mustDecodeBody decodes the JSON response body into a map and closes the body.
func mustDecodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("mustDecodeBody: %v", err)
	}
	return result
}
