package orca

import (
	"context"
	"strings"
	"testing"
)

func TestOpenStoreRequiresPostgres(t *testing.T) {
	store, err := OpenStore(context.Background(), "")
	if err == nil || store != nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("OpenStore without PostgreSQL should fail clearly; store=%v err=%v", store, err)
	}
}

func TestCloneStatePreservesPrivatePasswordHashes(t *testing.T) {
	state := NewState()
	state.Users["user-a"] = User{ID: "user-a", Email: "owner@example.test", PasswordHash: "private-hash"}

	copy, err := cloneState(state)
	if err != nil {
		t.Fatal(err)
	}
	if got := copy.Users["user-a"].PasswordHash; got != "private-hash" {
		t.Fatalf("cloneState lost the private password hash: got %q", got)
	}
}
