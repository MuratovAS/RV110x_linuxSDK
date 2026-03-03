package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	cookieName      = "uipm_session"
	sessionDuration = 24 * time.Hour

	maxLoginAttempts     = 5
	lockoutDuration      = 5 * time.Minute
	bruteForceCleanupTTL = 15 * time.Minute
)

var (
	sessions   = map[string]time.Time{}
	sessionsMu sync.Mutex
)

// ipAttempt tracks failed login attempts per client IP.
type ipAttempt struct {
	failures    int
	lockedUntil time.Time
}

var (
	loginAttempts   = map[string]*ipAttempt{}
	loginAttemptsMu sync.Mutex
)

func init() {
	// Periodically remove stale brute-force tracking entries.
	go func() {
		ticker := time.NewTicker(bruteForceCleanupTTL)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			loginAttemptsMu.Lock()
			for ip, a := range loginAttempts {
				if now.After(a.lockedUntil.Add(bruteForceCleanupTTL)) {
					delete(loginAttempts, ip)
				}
			}
			loginAttemptsMu.Unlock()
		}
	}()

	// Periodically evict expired sessions from memory.
	go func() {
		ticker := time.NewTicker(sessionDuration)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			sessionsMu.Lock()
			for token, exp := range sessions {
				if now.After(exp) {
					delete(sessions, token)
				}
			}
			sessionsMu.Unlock()
		}
	}()
}

// getClientIP returns the remote IP without the port.
func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isLoginLocked returns true if the IP is currently in a lockout window.
// It also resets the state when the lockout has naturally expired.
func isLoginLocked(ip string) bool {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	a := loginAttempts[ip]
	if a == nil {
		return false
	}
	if !a.lockedUntil.IsZero() && time.Now().Before(a.lockedUntil) {
		return true
	}
	// Lockout expired — clean up so the IP gets a fresh start.
	if !a.lockedUntil.IsZero() {
		delete(loginAttempts, ip)
	}
	return false
}

// recordFailedLogin increments the failure counter for the IP and applies a
// lockout once maxLoginAttempts is reached.
func recordFailedLogin(ip string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	a := loginAttempts[ip]
	if a == nil {
		a = &ipAttempt{}
		loginAttempts[ip] = a
	}
	// Reset if the previous lockout already expired.
	if !a.lockedUntil.IsZero() && time.Now().After(a.lockedUntil) {
		a.failures = 0
		a.lockedUntil = time.Time{}
	}
	a.failures++
	if a.failures >= maxLoginAttempts {
		a.lockedUntil = time.Now().Add(lockoutDuration)
	}
}

// resetLoginAttempts clears the failure state for the IP on a successful login.
func resetLoginAttempts(ip string) {
	loginAttemptsMu.Lock()
	delete(loginAttempts, ip)
	loginAttemptsMu.Unlock()
}

// invalidateAllSessions drops every active session cookie.
// Called when a password is set or cleared so existing tokens stop working.
func invalidateAllSessions() {
	sessionsMu.Lock()
	sessions = map[string]time.Time{}
	sessionsMu.Unlock()
}

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

	ip := getClientIP(r)
	if isLoginLocked(ip) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	cfgMu.RLock()
	hash := cfg.System.PasswordHash
	salt := cfg.System.PasswordSalt
	cfgMu.RUnlock()

	if hash != "" {
		computed := hashPassword(req.Password, salt)
		if subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) != 1 {
			recordFailedLogin(ip)
			http.Error(w, "invalid password", http.StatusUnauthorized)
			return
		}
	}

	resetLoginAttempts(ip)

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
