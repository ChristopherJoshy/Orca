package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
)

func approvalWire(v Approval) map[string]any {
	title := v.Action
	if title == "" {
		title = "Review proposed action"
	}
	return map[string]any{"id": v.ID, "title": title, "detail": fmt.Sprint(v.Inputs), "target": v.ResourceID, "resourceId": v.ResourceID, "createdAt": v.CreatedAt, "status": v.Status, "action": v.Action, "inputs": v.Inputs, "executionResult": v.ExecutionResult}
}
func (a *API) listApprovals(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role == "customer" {
		writeErr(w, ErrForbidden)
		return
	}
	items := []any{}
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		for _, v := range st.Approvals {
			items = append(items, approvalWire(v))
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *API) decideApproval(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req struct{ Decision string }
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	var result Approval
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		v, ok := st.Approvals[id]
		if !ok {
			return ErrNotFound
		}
		if v.Status != "Pending" || !v.ExpiresAt.After(time.Now()) {
			return ErrConflict
		}
		if v.ActorUserID != auth.Actor.UserID && auth.Actor.Role != "admin" {
			return ErrForbidden
		}
		if req.Decision != "approve" && req.Decision != "reject" {
			return fmt.Errorf("%w: decision must be approve or reject", ErrValidation)
		}
		now := time.Now().UTC()
		var executionResult map[string]any
		if req.Decision == "approve" {
			var err error
			executionResult, err = executeApprovedAction(st, auth.Actor, v, now)
			if err != nil {
				return err
			}
		}
		v.DecidedBy = auth.Actor.UserID
		v.DecidedAt = &now
		if req.Decision == "approve" {
			v.Status = "Approved"
			v.ExecutionResult = executionResult
		} else {
			v.Status = "Rejected"
		}
		st.Approvals[id] = v
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "approval."+req.Decision, "approval", id, map[string]any{"action": v.Action, "inputsHash": v.InputsHash, "resourceVersion": v.ResourceVersion, "executionResult": executionResult}))
		result = v
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, approvalWire(result))
}
func (a *API) listAgentRuns(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	items := []any{}
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		for _, v := range st.AgentRuns {
			if auth.Actor.Role != "customer" || v.UserID == auth.Actor.UserID {
				items = append(items, v)
			}
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *API) startAgentRun(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	var req struct {
		Message        string
		ConversationID string
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Message == "" || len(req.Message) > 8000 {
		writeErr(w, fmt.Errorf("%w: message is required and must be under 8000 characters", ErrValidation))
		return
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	conversation := req.ConversationID
	if conversation == "" {
		conversation = id
	}
	now := time.Now().UTC()
	status := "Queued"
	message := "Copilot request accepted."
	if a.Config.GroqAPIKey == "" {
		status = "Failed"
		message = "Copilot is not configured for this workspace. No business action was taken."
	}
	run := AgentRun{ID: id, BusinessID: auth.Actor.BusinessID, UserID: auth.Actor.UserID, ConversationID: conversation, Status: status, Message: message, Transcript: []any{map[string]any{"role": "user", "content": req.Message}}, Outcome: map[string]any{}, CreatedAt: now, UpdatedAt: now}
	err = a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error { st.AgentRuns[id] = run; return nil })
	if err != nil {
		writeErr(w, err)
		return
	}
	if a.Config.GroqAPIKey != "" {
		runContext := a.registerAgentExecution(id)
		go func() {
			defer a.finishAgentExecution(id)
			a.runGroqAgent(runContext, auth.Actor, run)
		}()
	}
	writeJSON(w, 202, run)
}
func (a *API) getAgentRun(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	id := chi.URLParam(r, "id")
	var run AgentRun
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		run, ok = st.AgentRuns[id]
		if !ok {
			return ErrNotFound
		}
		if auth.Actor.Role == "customer" && run.UserID != auth.Actor.UserID {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, run)
}
func (a *API) cancelAgentRun(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	id := chi.URLParam(r, "id")
	var run AgentRun
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		run, ok = st.AgentRuns[id]
		if !ok {
			return ErrNotFound
		}
		if run.UserID != auth.Actor.UserID && auth.Actor.Role != "admin" {
			return ErrForbidden
		}
		if run.Status == "Completed" || run.Status == "Failed" {
			return ErrConflict
		}
		run.Status = "Cancelled"
		run.UpdatedAt = time.Now().UTC()
		run.Message = "Run cancelled. No further actions will execute."
		st.AgentRuns[id] = run
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	a.cancelAgentExecution(id)
	writeJSON(w, 200, run)
}
func (a *API) requestExport(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role == "customer" && auth.Actor.CustomerID == "" {
		writeErr(w, ErrForbidden)
		return
	}
	var req struct {
		Kind    string
		Format  string
		Filters map[string]any
	}
	if r.ContentLength > 0 {
		if err := decodeBody(r, &req); err != nil {
			writeErr(w, err)
			return
		}
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	job := ExportJob{ID: id, BusinessID: auth.Actor.BusinessID, RequestedBy: auth.Actor.UserID, Status: "Queued", Filters: req.Filters, Manifest: map[string]any{}, CreatedAt: now}
	if job.Filters == nil {
		job.Filters = map[string]any{"kind": "full"}
	}
	err = a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		st.Exports[id] = job
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "export.requested", "export", id, job.Filters))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: auth.Actor.BusinessID, Type: "export.requested", IdempotencyKey: "export.requested:" + id, Payload: map[string]any{"exportId": id}, CreatedAt: now})
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	go a.buildExport(auth.Actor, job)
	writeJSON(w, 202, job)
}
func (a *API) getExport(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	id := chi.URLParam(r, "id")
	var job ExportJob
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		job, ok = st.Exports[id]
		if !ok {
			return ErrNotFound
		}
		if auth.Actor.Role == "customer" && job.RequestedBy != auth.Actor.UserID {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, job)
}
func (a *API) downloadExport(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	id := chi.URLParam(r, "id")
	var job ExportJob
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		job, ok = st.Exports[id]
		if !ok {
			return ErrNotFound
		}
		if auth.Actor.Role == "customer" && job.RequestedBy != auth.Actor.UserID {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	if job.Status != "Complete" || job.ExpiresAt == nil || !job.ExpiresAt.After(time.Now()) {
		writeJSON(w, 409, apiError{Message: "Export is still preparing or has expired"})
		return
	}
	f, err := os.Open(job.ArchivePath)
	if err != nil {
		writeErr(w, ErrNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=orca-export-%s.zip", id))
	_, _ = io.Copy(w, f)
}
func (a *API) buildExport(actor Actor, job ExportJob) {
	var state *State
	err := a.Store.ReadFor(context.Background(), actor.BusinessID, func(s *State) error { state = s; return nil })
	if err != nil {
		a.failExport(actor.BusinessID, job.ID, err)
		return
	}
	if actor.Role == "customer" {
		state = customerExportState(state, actor)
	}
	path, manifest, err := writeExportZip(state, job)
	if err != nil {
		a.failExport(actor.BusinessID, job.ID, err)
		return
	}
	_ = a.Store.UpdateFor(context.Background(), actor.BusinessID, func(st *State) error {
		current, ok := st.Exports[job.ID]
		if !ok || current.Status == "Expired" {
			_ = os.Remove(path)
			return nil
		}
		expires := time.Now().UTC().Add(24 * time.Hour)
		current.Status = "Complete"
		current.SnapshotAt = &manifest.GeneratedAt
		current.Manifest = manifest.Data
		current.ArchivePath = path
		current.ExpiresAt = &expires
		st.Exports[job.ID] = current
		return nil
	})
}
func (a *API) failExport(businessID, id string, err error) {
	_ = a.Store.UpdateFor(context.Background(), businessID, func(st *State) error {
		job, ok := st.Exports[id]
		if ok {
			job.Status = "Failed"
			job.Manifest = map[string]any{"error": "Export generation failed"}
			st.Exports[id] = job
		}
		return nil
	})
}

type exportManifest struct {
	GeneratedAt time.Time
	Data        map[string]any
}

func customerExportState(state *State, actor Actor) *State {
	out := NewState()
	out.Businesses[actor.BusinessID] = state.Businesses[actor.BusinessID]
	for id, v := range state.Customers {
		if id == actor.CustomerID {
			out.Customers[id] = v
		}
	}
	for id, v := range state.Subscriptions {
		if v.CustomerID == actor.CustomerID {
			out.Subscriptions[id] = v
		}
	}
	for id, v := range state.Invoices {
		if v.CustomerID == actor.CustomerID {
			out.Invoices[id] = v
		}
	}
	for id, v := range state.Payments {
		if v.CustomerID == actor.CustomerID {
			out.Payments[id] = v
		}
	}
	for key, v := range state.Allocations {
		if _, ok := out.Payments[v.PaymentID]; ok {
			out.Allocations[key] = v
		}
	}
	return out
}
func (a *API) listMCPConnections(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role != "admin" {
		writeErr(w, ErrForbidden)
		return
	}
	items := []any{}
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		for _, v := range st.OAuthGrants {
			items = append(items, v)
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *API) createMCPConnection(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req struct {
		ClientID string
		Scopes   []string
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.ClientID == "" {
		writeErr(w, fmt.Errorf("%w: clientId is required", ErrValidation))
		return
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	grant := OAuthGrant{ID: id, BusinessID: auth.Actor.BusinessID, UserID: auth.Actor.UserID, ClientID: req.ClientID, Scopes: req.Scopes, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)}
	err = a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error { st.OAuthGrants[id] = grant; return nil })
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, grant)
}
func (a *API) revokeMCPConnection(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireAdmin(auth); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	now := time.Now().UTC()
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		v, ok := st.OAuthGrants[id]
		if !ok {
			return ErrNotFound
		}
		v.RevokedAt = &now
		st.OAuthGrants[id] = v
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

var _ = json.Marshal
var _ = filepath.Clean
