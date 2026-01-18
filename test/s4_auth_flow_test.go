package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"day.local/gee"
	"day.local/gee/middleware"
	"day.local/internal/platform/auth"
	"day.local/internal/platform/httpmiddleware"
)

func TestS4_LoginAndAuthFlow(t *testing.T) {
	ts, err := auth.NewHS256Service("secret", "issuer", time.Hour)
	if err != nil {
		t.Fatalf("NewHS256Service: %v", err)
	}

	r := gee.New()
	r.Use(gee.Recovery(), middleware.ReqID())

	type loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	api := r.Group("/api/v1")
	api.POST("/login", func(ctx *gee.Context) {
		var req loginReq
		if err := ctx.BindJSON(&req); err != nil {
			return
		}

		var userID, role string
		switch {
		case req.Username == "user" && req.Password == "user":
			userID, role = "2", "user"
		default:
			ctx.AbortWithError(http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := ts.Sign(userID, role)
		if err != nil {
			ctx.AbortWithError(http.StatusBadGateway, "sign failed")
			return
		}
		ctx.JSON(http.StatusOK, map[string]string{"token": token})
	})

	users := api.Group("/users")
	users.Use(httpmiddleware.AuthRequired(ts))
	users.GET("/me", func(ctx *gee.Context) {
		id, ok := auth.GetIdentity(ctx.Req.Context())
		if !ok {
			ctx.AbortWithError(http.StatusInternalServerError, "missing identity")
			return
		}
		ctx.JSON(http.StatusOK, map[string]string{"user_id": id.UserID, "role": id.Role})
	})

	// login should not require Authorization
	body, _ := json.Marshal(loginReq{Username: "user", Password: "user"})
	loginReqHTTP := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(body))
	loginReqHTTP.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, loginReqHTTP)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status: got %d, want %d, body=%q", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}
	var loginResp map[string]string
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	token := loginResp["token"]
	if token == "" {
		t.Fatalf("login token is empty, raw=%q", loginRec.Body.String())
	}

	// protected route without token should be 401 (and not panic)
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	meRec := httptest.NewRecorder()
	r.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("me status without token: got %d, want %d, body=%q", meRec.Code, http.StatusUnauthorized, meRec.Body.String())
	}

	// protected route with token should be 200 and include identity
	meReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	meReq2.Header.Set("Authorization", "Bearer "+token)
	meRec2 := httptest.NewRecorder()
	r.ServeHTTP(meRec2, meReq2)
	if meRec2.Code != http.StatusOK {
		t.Fatalf("me status with token: got %d, want %d, body=%q", meRec2.Code, http.StatusOK, meRec2.Body.String())
	}
	var meResp map[string]string
	if err := json.NewDecoder(meRec2.Body).Decode(&meResp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResp["user_id"] != "2" {
		t.Fatalf("user_id: got %q, want %q", meResp["user_id"], "2")
	}
	if meResp["role"] != "user" {
		t.Fatalf("role: got %q, want %q", meResp["role"], "user")
	}
}
