package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"day.local/gee"
	"day.local/gee/middleware"
	"day.local/internal/app/shortlink/httpapi"
	"day.local/internal/app/shortlink/repo"
	"day.local/internal/app/shortlink/stats"
	"day.local/internal/platform/auth"
	"day.local/internal/platform/db"
	"day.local/internal/platform/httpmiddleware"
)

// setupTestServer creates a test server with all shortlink routes
func setupTestServer(t *testing.T) (*gee.Engine, *repo.ShortlinksRepo, *repo.UsersRepo, auth.TokenService) {
	t.Helper()

	// Connect to test database
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use environment variable or default
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://days:days@localhost:5432/days?sslmode=disable"
	}
	dbPool, err := db.New(dbCtx, dsn)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to test database: %v", err)
	}
	t.Cleanup(func() { dbPool.Close() })

	// Ping database
	if err := dbPool.Ping(dbCtx); err != nil {
		t.Skipf("Skipping test: cannot ping test database: %v", err)
	}

	// Create repos
	slRepo := repo.NewShortlinksRepo(dbPool, nil, nil)
	usersRepo := repo.NewUsersRepo(dbPool)

	// Create JWT service
	ts, err := auth.NewHS256Service("test-secret-key", "test-issuer", time.Hour)
	if err != nil {
		t.Fatalf("NewHS256Service: %v", err)
	}

	// Create engine
	r := gee.New()
	r.Use(gee.Recovery(), middleware.ReqID(), middleware.AccessLog())

	// Register routes
	api := r.Group("/api/v1")
	httpapi.RegisterAPIRoutes(api, slRepo, usersRepo, ts, nil)
	collector := stats.NewChannelCollector(100)
	t.Cleanup(func() { collector.Close() })
	httpapi.RegisterPublicRoutes(r, slRepo, collector, nil)

	// Add healthz for route priority test
	r.GET("/healthz", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "ok")
	})

	// Admin routes
	admin := api.Group("/admin")
	admin.Use(httpmiddleware.AuthRequired(ts), httpmiddleware.RequireRole("admin"))
	admin.POST("/shortlinks/:code/disable", httpapi.NewDisablesHandler(slRepo))

	return r, slRepo, usersRepo, ts
}

// TestRoutePriority tests that static routes take priority over wildcard routes
func TestRoutePriority(t *testing.T) {
	r := gee.New()

	// Register routes in different order to test priority
	r.GET("/:code", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "wildcard:%s", ctx.Param("code"))
	})
	r.GET("/healthz", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "%s", "static:healthz")
	})
	r.GET("/api", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "%s", "static:api")
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/healthz", "static:healthz"},
		{"/api", "static:api"},
		{"/abc123", "wildcard:abc123"},
		{"/xyz", "wildcard:xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != tt.expected {
				t.Errorf("body: got %q, want %q", rec.Body.String(), tt.expected)
			}
		})
	}
}

// TestUserRegistration tests user registration endpoint
func TestUserRegistration(t *testing.T) {
	r, _, _, _ := setupTestServer(t)

	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		{"valid registration", "testuser_" + time.Now().Format("150405"), "password123", http.StatusCreated},
		{"short username", "ab", "password123", http.StatusBadRequest},
		{"short password", "validuser", "short", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{
				"username": tt.username,
				"password": tt.password,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestLoginAndCreateShortlink tests the full flow: login -> create shortlink
func TestLoginAndCreateShortlink(t *testing.T) {
	r, _, _, _ := setupTestServer(t)

	// First register a user
	username := "shortlink_test_" + time.Now().Format("150405")
	password := "testpassword123"

	regBody, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	r.ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d, body=%s", regRec.Code, regRec.Body.String())
	}

	// Login
	loginBody, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d, body=%s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp map[string]string
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	token := loginResp["token"]
	if token == "" {
		t.Fatalf("token is empty")
	}

	// Create shortlink (authenticated)
	createBody, _ := json.Marshal(map[string]string{
		"url": "https://example.com/test-" + time.Now().Format("150405"),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/shortlinks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("create shortlink failed: %d, body=%s", createRec.Code, createRec.Body.String())
	}

	var createResp map[string]string
	if err := json.NewDecoder(createRec.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	code := createResp["code"]
	if code == "" {
		t.Fatalf("code is empty")
	}
	t.Logf("Created shortlink with code: %s", code)

	// Test redirect
	redirectReq := httptest.NewRequest(http.MethodGet, "/"+code, nil)
	redirectRec := httptest.NewRecorder()
	r.ServeHTTP(redirectRec, redirectReq)

	if redirectRec.Code != http.StatusFound {
		t.Errorf("redirect status: got %d, want %d", redirectRec.Code, http.StatusFound)
	}
	location := redirectRec.Header().Get("Location")
	if location == "" {
		t.Errorf("Location header is empty")
	}
	t.Logf("Redirect location: %s", location)

	// Test get my shortlinks
	mineReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/mine", nil)
	mineReq.Header.Set("Authorization", "Bearer "+token)
	mineRec := httptest.NewRecorder()
	r.ServeHTTP(mineRec, mineReq)

	if mineRec.Code != http.StatusOK {
		t.Errorf("get mine failed: %d, body=%s", mineRec.Code, mineRec.Body.String())
	}

	// Test remove from my list
	removeReq := httptest.NewRequest(http.MethodDelete, "/api/v1/users/mine/"+code, nil)
	removeReq.Header.Set("Authorization", "Bearer "+token)
	removeRec := httptest.NewRecorder()
	r.ServeHTTP(removeRec, removeReq)

	if removeRec.Code != http.StatusOK {
		t.Errorf("remove from mine failed: %d, body=%s", removeRec.Code, removeRec.Body.String())
	}
}

// TestAnonymousCreateShortlink tests creating shortlink without authentication
func TestAnonymousCreateShortlink(t *testing.T) {
	r, _, _, _ := setupTestServer(t)

	createBody, _ := json.Marshal(map[string]string{
		"url": "https://example.com/anonymous-" + time.Now().Format("150405"),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/shortlinks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("anonymous create shortlink failed: %d, body=%s", createRec.Code, createRec.Body.String())
	}

	var createResp map[string]string
	if err := json.NewDecoder(createRec.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp["code"] == "" {
		t.Fatalf("code is empty")
	}
	t.Logf("Anonymous shortlink code: %s", createResp["code"])
}

// TestInvalidURL tests creating shortlink with invalid URL
func TestInvalidURL(t *testing.T) {
	r, _, _, _ := setupTestServer(t)

	tests := []struct {
		name string
		url  string
	}{
		{"empty url", ""},
		{"no scheme", "example.com"},
		{"invalid scheme", "ftp://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createBody, _ := json.Marshal(map[string]string{
				"url": tt.url,
			})
			createReq := httptest.NewRequest(http.MethodPost, "/api/v1/shortlinks", bytes.NewReader(createBody))
			createReq.Header.Set("Content-Type", "application/json")
			createRec := httptest.NewRecorder()
			r.ServeHTTP(createRec, createReq)

			if createRec.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want %d, body=%s", createRec.Code, http.StatusBadRequest, createRec.Body.String())
			}
		})
	}
}

// TestAdminDisableShortlink tests that only admin can disable shortlinks
func TestAdminDisableShortlink(t *testing.T) {
	r, _, _, ts := setupTestServer(t)

	// Create a shortlink first
	createBody, _ := json.Marshal(map[string]string{
		"url": "https://example.com/admin-test-" + time.Now().Format("150405"),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/shortlinks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("create shortlink failed: %d", createRec.Code)
	}

	var createResp map[string]string
	json.NewDecoder(createRec.Body).Decode(&createResp)
	code := createResp["code"]

	// Test 1: No auth - should fail
	disableReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shortlinks/"+code+"/disable", nil)
	disableRec := httptest.NewRecorder()
	r.ServeHTTP(disableRec, disableReq)

	if disableRec.Code != http.StatusUnauthorized {
		t.Errorf("no auth: got %d, want %d", disableRec.Code, http.StatusUnauthorized)
	}

	// Test 2: User role - should fail with 403
	userToken, _ := ts.Sign("1", "user")
	disableReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shortlinks/"+code+"/disable", nil)
	disableReq2.Header.Set("Authorization", "Bearer "+userToken)
	disableRec2 := httptest.NewRecorder()
	r.ServeHTTP(disableRec2, disableReq2)

	if disableRec2.Code != http.StatusForbidden {
		t.Errorf("user role: got %d, want %d, body=%s", disableRec2.Code, http.StatusForbidden, disableRec2.Body.String())
	}

	// Test 3: Admin role - should succeed
	adminToken, _ := ts.Sign("1", "admin")
	disableReq3 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shortlinks/"+code+"/disable", nil)
	disableReq3.Header.Set("Authorization", "Bearer "+adminToken)
	disableRec3 := httptest.NewRecorder()
	r.ServeHTTP(disableRec3, disableReq3)

	if disableRec3.Code != http.StatusOK {
		t.Errorf("admin role: got %d, want %d, body=%s", disableRec3.Code, http.StatusOK, disableRec3.Body.String())
	}

	// Test 4: Redirect should fail after disable
	redirectReq := httptest.NewRequest(http.MethodGet, "/"+code, nil)
	redirectRec := httptest.NewRecorder()
	r.ServeHTTP(redirectRec, redirectReq)

	if redirectRec.Code != http.StatusNotFound {
		t.Errorf("redirect after disable: got %d, want %d", redirectRec.Code, http.StatusNotFound)
	}
}

// TestUserCannotAccessAdminRoutes tests that regular users cannot access admin routes
func TestUserCannotAccessAdminRoutes(t *testing.T) {
	r, _, _, ts := setupTestServer(t)

	userToken, _ := ts.Sign("1", "user")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/shortlinks/abc/disable", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCreateShortlinkWithCustomCode(t *testing.T) {
	r, _, _, _ := setupTestServer(t)

	code := "C" + strconv.FormatInt(time.Now().UnixNano()%1_000_000_000_000, 36)
	url1 := "https://example.com/custom-code-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	body1, _ := json.Marshal(map[string]string{
		"url":  url1,
		"code": code,
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/shortlinks", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("create with custom code failed: %d, body=%s", rec1.Code, rec1.Body.String())
	}
	var resp1 map[string]string
	if err := json.NewDecoder(rec1.Body).Decode(&resp1); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp1["code"] != code {
		t.Fatalf("code mismatch: got %q, want %q", resp1["code"], code)
	}

	// same code + different url => conflict
	body2, _ := json.Marshal(map[string]string{
		"url":  "https://example.com/custom-code-other-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		"code": code,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/shortlinks", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected conflict for duplicate code: got %d, body=%s", rec2.Code, rec2.Body.String())
	}

	// same url + different code => conflict (url is unique in this app)
	body3, _ := json.Marshal(map[string]string{
		"url":  url1,
		"code": "C" + strconv.FormatInt(time.Now().UnixNano()%1_000_000_000_000, 36),
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/shortlinks", bytes.NewReader(body3))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusConflict {
		t.Fatalf("expected conflict for same url with different code: got %d, body=%s", rec3.Code, rec3.Body.String())
	}
}
