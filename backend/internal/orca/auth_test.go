package orca

import "testing"

func TestArgon2PasswordHashAndPolicy(t *testing.T) {
	if err := validatePassword("lowercase1!"); err == nil {
		t.Fatal("expected uppercase requirement")
	}
	if err := validatePassword("PASSWORD1!"); err == nil {
		t.Fatal("expected lowercase requirement")
	}
	if err := validatePassword("Password12"); err == nil {
		t.Fatal("expected special character requirement")
	}
	password := "Password!2026"
	hash, err := passwordHash(password)
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, password) {
		t.Fatal("valid password did not verify")
	}
	if verifyPassword(hash, password+"x") {
		t.Fatal("incorrect password verified")
	}
}
