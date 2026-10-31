package orca

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

var (
	ErrUnauthorized     = errors.New("authentication required")
	ErrForbidden        = errors.New("permission denied")
	ErrNotFound         = errors.New("record not found")
	ErrConflict         = errors.New("record already exists or state conflicts")
	ErrValidation       = errors.New("request validation failed")
	ErrApprovalRequired = errors.New("approval required")
)

type Authenticated struct {
	Actor     Actor
	TokenHash string
	CSRFHash  string
	ExpiresAt time.Time
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func validateEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || !strings.Contains(value, "@") || len(value) > 254 {
		return "", fmt.Errorf("%w: enter a valid email address", ErrValidation)
	}
	return normalizeEmail(value), nil
}

func validatePassword(password string) error {
	if len(password) < 9 {
		return fmt.Errorf("%w: password must be at least 9 characters", ErrValidation)
	}
	var upper, lower, special bool
	for _, r := range password {
		if r >= 'A' && r <= 'Z' {
			upper = true
		}
		if r >= 'a' && r <= 'z' {
			lower = true
		}
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			special = true
		}
	}
	if !upper || !lower || !special {
		return fmt.Errorf("%w: password needs uppercase, lowercase, and a special character", ErrValidation)
	}
	return nil
}

func passwordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	const memory, iterations, parallelism = 64 * 1024, 3, 2
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, iterations, parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(encoded, password string) bool {
	var memory, iterations, parallelism uint32
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil || memory < 8*1024 || memory > 256*1024 || iterations < 1 || iterations > 10 || parallelism < 1 || parallelism > 8 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expected)))
	if len(actual) != len(expected) {
		return false
	}
	var diff byte
	for i := range actual {
		diff |= actual[i] ^ expected[i]
	}
	return diff == 0
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	hexv := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexv[0:8], hexv[8:12], hexv[12:16], hexv[16:20], hexv[20:]), nil
}

func (s *Store) LookupUser(ctx context.Context, email string) (User, []Membership, error) {
	normalized := normalizeEmail(email)
	if s.pool != nil {
		var user User
		err := s.pool.QueryRow(ctx, "SELECT id::text,email,email_normalized,password_hash,(email_verified_at IS NOT NULL),created_at FROM users WHERE email_normalized=$1", normalized).Scan(&user.ID, &user.Email, &user.EmailNormalized, &user.PasswordHash, &user.Verified, &user.CreatedAt)
		if err != nil {
			return User{}, nil, ErrUnauthorized
		}
		rows, err := s.pool.Query(ctx, "SELECT business_id::text,user_id::text,role,COALESCE(customer_id::text,''),revoked_at FROM memberships WHERE user_id=$1 AND revoked_at IS NULL ORDER BY created_at", user.ID)
		if err != nil {
			return User{}, nil, err
		}
		defer rows.Close()
		var members []Membership
		for rows.Next() {
			var m Membership
			if err := rows.Scan(&m.BusinessID, &m.UserID, &m.Role, &m.CustomerID, &m.RevokedAt); err != nil {
				return User{}, nil, err
			}
			members = append(members, m)
		}
		if rows.Err() != nil {
			return User{}, nil, rows.Err()
		}
		if len(members) == 0 {
			return User{}, nil, ErrUnauthorized
		}
		return user, members, nil
	}
	var user User
	var members []Membership
	err := s.Read(ctx, func(st *State) error {
		for _, u := range st.Users {
			if u.EmailNormalized == normalized {
				user = u
				break
			}
		}
		if user.ID == "" {
			return ErrUnauthorized
		}
		for _, m := range st.Memberships {
			if m.UserID == user.ID && m.RevokedAt == nil {
				members = append(members, m)
			}
		}
		return nil
	})
	if err != nil {
		return User{}, nil, err
	}
	if len(members) == 0 {
		return User{}, nil, ErrUnauthorized
	}
	return user, members, nil
}

func (s *Store) ResolveSession(ctx context.Context, token string) (Authenticated, error) {
	if token == "" {
		return Authenticated{}, ErrUnauthorized
	}
	digest := tokenDigest(token)
	if s.pool != nil {
		var a Authenticated
		err := s.pool.QueryRow(ctx, `SELECT s.user_id::text,s.business_id::text,COALESCE(m.customer_id::text,''),m.role,encode(s.csrf_hash,'hex'),s.expires_at FROM sessions s JOIN memberships m ON m.user_id=s.user_id AND m.business_id=s.business_id WHERE s.token_hash=$1 AND s.expires_at>now() AND m.revoked_at IS NULL`, mustHex(digest)).Scan(&a.Actor.UserID, &a.Actor.BusinessID, &a.Actor.CustomerID, &a.Actor.Role, &a.CSRFHash, &a.ExpiresAt)
		if err != nil {
			return Authenticated{}, ErrUnauthorized
		}
		a.TokenHash = digest
		return a, nil
	}
	var a Authenticated
	err := s.Read(ctx, func(st *State) error {
		session, ok := st.Sessions[digest]
		if !ok || !session.ExpiresAt.After(time.Now()) {
			return ErrUnauthorized
		}
		m, ok := st.Memberships[membershipKey(session.BusinessID, session.UserID)]
		if !ok || m.RevokedAt != nil {
			return ErrUnauthorized
		}
		a = Authenticated{Actor: Actor{UserID: session.UserID, BusinessID: session.BusinessID, CustomerID: m.CustomerID, Role: m.Role}, TokenHash: digest, CSRFHash: session.CSRFToken, ExpiresAt: session.ExpiresAt}
		return nil
	})
	return a, err
}

func mustHex(s string) []byte { b, _ := hex.DecodeString(s); return b }

func (s *Store) CreateAccount(ctx context.Context, user User, business Business, session Session, sessionHash string) error {
	return s.Update(ctx, func(st *State) error {
		for _, existing := range st.Users {
			if existing.EmailNormalized == user.EmailNormalized {
				return ErrConflict
			}
		}
		st.Users[user.ID] = user
		st.Businesses[business.ID] = business
		st.Memberships[membershipKey(business.ID, user.ID)] = Membership{BusinessID: business.ID, UserID: user.ID, Role: "admin"}
		st.Sessions[sessionHash] = session
		return nil
	})
}

func (s *Store) InsertSession(ctx context.Context, session Session, sessionHash string) error {
	return s.UpdateFor(ctx, session.BusinessID, func(st *State) error {
		if m, ok := st.Memberships[membershipKey(session.BusinessID, session.UserID)]; !ok || m.RevokedAt != nil {
			return ErrUnauthorized
		}
		st.Sessions[sessionHash] = session
		return nil
	})
}

func (s *Store) RevokeSession(ctx context.Context, businessID, tokenHash string) error {
	return s.UpdateFor(ctx, businessID, func(st *State) error { delete(st.Sessions, tokenHash); return nil })
}
