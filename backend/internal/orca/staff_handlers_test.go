package orca

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInviteStaffAddsExistingGlobalUserToOnlyCurrentBusiness(t *testing.T) {
	api, admin := newAgentToolTestAPI()
	other := User{ID: "existing-user", Email: "existing@example.test", EmailNormalized: "existing@example.test", Verified: false, CreatedAt: time.Now().UTC()}
	api.Store.state.Users[other.ID] = other
	api.Store.state.Memberships[membershipKey("biz-b", other.ID)] = Membership{BusinessID: "biz-b", UserID: other.ID, Role: "admin"}

	request := httptest.NewRequest("POST", "/api/v1/staff/invitations", strings.NewReader(`{"email":"existing@example.test","role":"staff"}`))
	response := httptest.NewRecorder()
	api.inviteStaff(response, request, Authenticated{Actor: admin})
	if response.Code != 201 {
		t.Fatalf("inviteStaff status = %d, body = %s", response.Code, response.Body.String())
	}

	if got := api.Store.state.Memberships[membershipKey("biz-a", other.ID)].Role; got != "staff" {
		t.Fatalf("new business role = %q, want staff", got)
	}
	if got := api.Store.state.Memberships[membershipKey("biz-b", other.ID)].Role; got != "admin" {
		t.Fatalf("existing business role changed to %q", got)
	}
	if len(api.Store.state.Audit) != 1 {
		t.Fatalf("audit events = %d, want 1", len(api.Store.state.Audit))
	}
	if got := api.Store.state.Audit[0].ResourceID; got != other.ID {
		t.Fatalf("audit resource ID = %q, want user UUID %q", got, other.ID)
	}
}

func TestInviteStaffRequiresAnExistingAccount(t *testing.T) {
	api, admin := newAgentToolTestAPI()

	request := httptest.NewRequest("POST", "/api/v1/staff/invitations", strings.NewReader(`{"name":"Pending account","email":"pending@example.test","role":"staff"}`))
	response := httptest.NewRecorder()
	api.inviteStaff(response, request, Authenticated{Actor: admin})
	if response.Code != 422 {
		t.Fatalf("inviteStaff status = %d, want 422; body = %s", response.Code, response.Body.String())
	}
	if len(api.Store.state.Memberships) != 1 {
		t.Fatalf("nonexistent account received a membership: %#v", api.Store.state.Memberships)
	}
}
