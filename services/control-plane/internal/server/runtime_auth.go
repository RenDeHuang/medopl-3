package server

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

func (app *controlPlaneApp) createUser(input map[string]any) (map[string]any, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	email := stringField(input, "email", "admin@medopl.cn")
	for _, existing := range app.auth.users {
		if strings.EqualFold(stringValue(existing["email"]), email) {
			return nil, errUserExists
		}
	}
	id := "usr-" + compactID(email+"-"+time.Now().UTC().Format("20060102150405.000000000"))
	password := stringField(input, "password", "")
	if password == "" {
		return nil, errors.New("missing_password")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	user := map[string]any{"id": id, "email": email, "accountId": stringField(input, "accountId", "acct-admin"), "role": stringField(input, "role", "owner"), "status": "active", "passwordHash": passwordHash}
	app.auth.users[id] = user
	return sanitizeUser(user), app.persistLocked()
}

func (app *controlPlaneApp) disableUser(input map[string]any) (map[string]any, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	id := stringField(input, "userId", "")
	user := app.auth.users[id]
	if user == nil {
		return nil, errUserNotFound
	}
	if stringValue(user["status"]) == "deleted" {
		return nil, errUserDeleted
	}
	if stringValue(user["role"]) == "admin" && stringValue(user["status"]) == "active" && app.activeAdminCountLocked() <= 1 {
		return nil, errLastActiveAdmin
	}
	user["status"] = "disabled"
	user["disabledAt"] = time.Now().UTC().Format(time.RFC3339)
	user["disabledBy"] = firstNonEmpty(stringField(input, "operatorUserId", ""), stringField(input, "disabledBy", ""), "usr-admin")
	user["disabledReason"] = stringField(input, "reason", "admin_disabled")
	app.revokeUserSessionsLocked(id)
	return sanitizeUser(user), app.persistLocked()
}

func (app *controlPlaneApp) softDeleteUser(input map[string]any) (map[string]any, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	id := stringField(input, "userId", "")
	user := app.auth.users[id]
	if user == nil {
		return nil, errUserNotFound
	}
	if stringValue(user["status"]) == "deleted" {
		return sanitizeUser(user), nil
	}
	if stringValue(user["role"]) == "admin" && stringValue(user["status"]) == "active" && app.activeAdminCountLocked() <= 1 {
		return nil, errLastActiveAdmin
	}
	user["status"] = "deleted"
	user["deletedAt"] = time.Now().UTC().Format(time.RFC3339)
	user["deletedBy"] = firstNonEmpty(stringField(input, "operatorUserId", ""), stringField(input, "deletedBy", ""), "usr-admin")
	user["deleteReason"] = stringField(input, "reason", "admin_deleted")
	app.revokeUserSessionsLocked(id)
	return sanitizeUser(user), app.persistLocked()
}

func (app *controlPlaneApp) activeAdminCountLocked() int {
	count := 0
	for _, user := range app.auth.users {
		if stringValue(user["role"]) == "admin" && stringValue(user["status"]) == "active" {
			count++
		}
	}
	return count
}

func (app *controlPlaneApp) revokeUserSessionsLocked(userID string) {
	for sessionID, session := range app.auth.sessions {
		if session.UserID == userID {
			delete(app.auth.sessions, sessionID)
		}
	}
}

func (app *controlPlaneApp) importBootstrapUsers() error {
	users, err := bootstrapUsersFromEnv()
	if err != nil {
		return err
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	app.dropLegacyOwnerUserLocked()
	for _, user := range users {
		app.upsertBootstrapUserLocked(user)
	}
	return app.persistLocked()
}

func (app *controlPlaneApp) dropLegacyOwnerUserLocked() {
	for id, user := range app.auth.users {
		if strings.EqualFold(stringValue(user["email"]), "owner@example.com") {
			delete(app.auth.users, id)
		}
	}
}

func (app *controlPlaneApp) upsertBootstrapUserLocked(seed map[string]any) {
	id := stringValue(seed["id"])
	for existingID, existing := range app.auth.users {
		if existingID == id || strings.EqualFold(stringValue(existing["email"]), stringValue(seed["email"])) {
			for key, value := range seed {
				if key == "passwordHash" && stringValue(existing["passwordHash"]) != "" {
					continue
				}
				if key == "id" {
					continue
				}
				existing[key] = value
			}
			return
		}
	}
	app.auth.users[id] = seed
}

func (app *controlPlaneApp) login(input map[string]any) (map[string]any, string, error) {
	email := strings.ToLower(strings.TrimSpace(stringField(input, "email", "")))
	password := stringField(input, "password", "")
	app.mu.Lock()
	defer app.mu.Unlock()
	for _, user := range app.auth.users {
		if strings.ToLower(stringValue(user["email"])) != email {
			continue
		}
		if stringValue(user["status"]) != "active" || !verifyPassword(password, stringValue(user["passwordHash"])) {
			return nil, "", errors.New("invalid_credentials")
		}
		return app.createSessionLocked(user)
	}
	return nil, "", errors.New("invalid_credentials")
}

func (app *controlPlaneApp) loginRateLimited(r *http.Request, input map[string]any) bool {
	key := loginFailureKey(r, input)
	app.mu.Lock()
	defer app.mu.Unlock()
	failure := app.auth.failures[key]
	if !failure.FirstAt.IsZero() && time.Since(failure.FirstAt) > 15*time.Minute {
		delete(app.auth.failures, key)
		return false
	}
	return failure.Count >= 5
}

func (app *controlPlaneApp) recordLoginFailure(r *http.Request, input map[string]any) {
	key := loginFailureKey(r, input)
	app.mu.Lock()
	defer app.mu.Unlock()
	failure := app.auth.failures[key]
	if failure.FirstAt.IsZero() || time.Since(failure.FirstAt) > 15*time.Minute {
		failure = loginFailure{FirstAt: time.Now().UTC()}
	}
	failure.Count++
	app.auth.failures[key] = failure
}

func (app *controlPlaneApp) clearLoginFailures(r *http.Request, input map[string]any) {
	key := loginFailureKey(r, input)
	app.mu.Lock()
	defer app.mu.Unlock()
	delete(app.auth.failures, key)
}

func loginFailureKey(r *http.Request, input map[string]any) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	email := strings.ToLower(strings.TrimSpace(stringField(input, "email", "")))
	return email + "|" + host
}

func (app *controlPlaneApp) operatorLogin() (map[string]any, string, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	for _, user := range app.auth.users {
		if stringValue(user["role"]) == "admin" && stringValue(user["status"]) == "active" {
			return app.createSessionLocked(user)
		}
	}
	operator := map[string]any{"id": "usr-operator", "email": "operator@opl.local", "accountId": "acct-operator", "role": "admin", "status": "active"}
	return app.createSessionLocked(operator)
}

func (app *controlPlaneApp) createSessionLocked(user map[string]any) (map[string]any, string, error) {
	sessionID, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return nil, "", err
	}
	sessionKey := sessionLookupKey(sessionID)
	app.auth.sessions[sessionKey] = sessionRecord{ID: sessionKey, UserID: stringValue(user["id"]), CSRF: csrf, ExpiresAt: time.Now().UTC().Add(12 * time.Hour)}
	if err := app.persistLocked(); err != nil {
		delete(app.auth.sessions, sessionKey)
		return nil, "", err
	}
	return map[string]any{"user": sanitizeUser(user), "csrfToken": csrf, "expiresAt": app.auth.sessions[sessionKey].ExpiresAt.Format(time.RFC3339)}, sessionID, nil
}

func (app *controlPlaneApp) session(r *http.Request) (map[string]any, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	sessionKey := sessionLookupKey(cookie.Value)
	session, ok := app.auth.sessions[sessionKey]
	if !ok || time.Now().UTC().After(session.ExpiresAt) {
		delete(app.auth.sessions, sessionKey)
		_ = app.persistLocked()
		return nil, false
	}
	user := app.auth.users[session.UserID]
	if user == nil || stringValue(user["status"]) != "active" {
		return nil, false
	}
	return map[string]any{"user": sanitizeUser(user), "csrfToken": session.CSRF, "expiresAt": session.ExpiresAt.Format(time.RFC3339)}, true
}

func (app *controlPlaneApp) sessionUserID(r *http.Request) string {
	user, ok := app.sessionUserContext(r)
	if !ok {
		return ""
	}
	return stringValue(user["id"])
}

func (app *controlPlaneApp) sessionUserContext(r *http.Request) (map[string]any, bool) {
	payload, ok := app.session(r)
	if !ok {
		return nil, false
	}
	user, _ := payload["user"].(map[string]any)
	return user, user != nil
}

func (app *controlPlaneApp) logout(r *http.Request) error {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	delete(app.auth.sessions, sessionLookupKey(cookie.Value))
	return app.persistLocked()
}

func (app *controlPlaneApp) sessionFactsLocked() controlPlaneRecordSet {
	output := controlPlaneRecordSet{}
	for id, session := range app.auth.sessions {
		output[id] = controlPlaneRecord{
			"id":        session.ID,
			"userId":    session.UserID,
			"csrf":      session.CSRF,
			"expiresAt": session.ExpiresAt.UTC().Format(time.RFC3339),
		}
	}
	return output
}
