package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	cookieName      = "uipm_session"
	sessionDuration = 24 * time.Hour
)

var (
	sessions   = map[string]time.Time{}
	sessionsMu sync.Mutex
)

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

func generateSalt() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt + password))
	return hex.EncodeToString(h.Sum(nil))
}

// isAuthenticated returns true if the request carries a valid session cookie,
// or if no password has been configured (open access).
func isAuthenticated(r *http.Request) bool {
	cfgMu.RLock()
	hash := cfg.System.PasswordHash
	cfgMu.RUnlock()
	if hash == "" {
		return true
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	exp, ok := sessions[cookie.Value]
	if !ok || time.Now().After(exp) {
		delete(sessions, cookie.Value)
		return false
	}
	return true
}

// authMiddleware wraps a handler and returns 401 when not authenticated.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})
}

// infoHandler returns public app info: auth status and firmware version.
// Does NOT itself require auth.
func infoHandler(w http.ResponseWriter, r *http.Request) {
	cfgMu.RLock()
	hash := cfg.System.PasswordHash
	theme := cfg.System.Theme
	cfgMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"passwordRequired": hash != "",
		"authenticated":    isAuthenticated(r),
		"version":          version,
		"theme":            theme,
	})
}

// loginHandler accepts POST { "password": "..." } and issues a session cookie.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	cfgMu.RLock()
	hash := cfg.System.PasswordHash
	salt := cfg.System.PasswordSalt
	cfgMu.RUnlock()

	if hash != "" && hashPassword(req.Password, salt) != hash {
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}

	token := generateToken()
	sessionsMu.Lock()
	sessions[token] = time.Now().Add(sessionDuration)
	sessionsMu.Unlock()

	setSessionCookie(w, token)
	w.WriteHeader(http.StatusNoContent)
}

// logoutHandler destroys the current session.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		sessionsMu.Lock()
		delete(sessions, cookie.Value)
		sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}
