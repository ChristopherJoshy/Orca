package orca

import (
	"errors"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// signupCustomer only grants portal access when a business already has a
// customer record with this verified email address. The submitted business
// ID selects the record; the email verification step proves ownership.
func (a *API) signupCustomer(w http.ResponseWriter, r *http.Request, emailValue, password, businessID string) {
	email, err := validateEmail(emailValue)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := validatePassword(password); err != nil {
		writeErr(w, err)
		return
	}
	hash, err := passwordHash(password)
	if err != nil {
		writeErr(w, err)
		return
	}
	userID, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	user := User{ID: userID, Email: strings.TrimSpace(emailValue), EmailNormalized: email, PasswordHash: hash, Verified: a.Config.Env == "development", CreatedAt: now}
	verificationToken, verificationHash := "", ""
	if !user.Verified {
		verificationToken, err = randomToken(32)
		if err != nil {
			writeErr(w, err)
			return
		}
		verificationHash = tokenDigest(verificationToken)
	}
	var business Business
	var member Membership
	err = a.Store.Update(r.Context(), func(st *State) error {
		for _, existing := range st.Users {
			if existing.EmailNormalized == email {
				return ErrConflict
			}
		}
		var ok bool
		business, ok = st.Businesses[businessID]
		if !ok {
			return ErrNotFound
		}
		var matched Customer
		for _, candidate := range st.Customers {
			if candidate.BusinessID == businessID && normalizeEmail(candidate.Email) == email {
				matched = candidate
				break
			}
		}
		if matched.ID == "" {
			return ErrNotFound
		}
		if matched.UserID != "" {
			return ErrConflict
		}
		matched.UserID = user.ID
		st.Customers[matched.ID] = matched
		st.Users[user.ID] = user
		member = Membership{BusinessID: businessID, UserID: user.ID, CustomerID: matched.ID, Role: "customer"}
		st.Memberships[membershipKey(businessID, user.ID)] = member
		if !user.Verified {
			st.EmailVerifications[verificationHash] = AuthToken{UserID: user.ID, ExpiresAt: now.Add(24 * time.Hour)}
		}
		return nil
	})
	if err != nil {
		// Do not reveal whether the business, contact, or account exists.
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrConflict) {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
		return
	}
	if !user.Verified {
		delivered := false
		if a.Config.ResendAPIKey != "" {
			link := strings.TrimRight(a.Config.PublicURL, "/") + "/?verify=" + url.QueryEscape(verificationToken)
			body := "<p>Verify your ORCA customer portal account:</p><p><a href=\"" + html.EscapeString(link) + "\">Verify email</a></p>"
			delivered = a.sendResendEmail(r.Context(), user.Email, "Verify your ORCA customer account", body, "orca-customer-verify:"+verificationHash) == nil
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "emailSent": delivered})
		return
	}
	token, csrf, session, err := a.issueSession(r.Context(), user, business, member)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.setSession(w, token, session.ExpiresAt)
	writeJSON(w, http.StatusCreated, map[string]any{"user": map[string]any{"id": user.ID, "email": user.Email, "verified": true}, "business": business, "csrfToken": csrf, "autoVerified": true})
}
