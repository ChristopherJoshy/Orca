package orca

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

func approvalWire(v Approval) map[string]any {
	label := approvalActionLabel(v.Action)
	return map[string]any{"id": v.ID, "title": label, "detail": approvalDetail(v), "target": v.ResourceID, "resourceId": v.ResourceID, "createdAt": v.CreatedAt, "status": v.Status, "action": label, "inputs": v.Inputs, "executionResult": v.ExecutionResult}
}

func approvalActionLabel(action string) string {
	switch action {
	case "confirm_invoice":
		return "Confirm invoice"
	case "cancel_invoice":
		return "Cancel invoice"
	case "send_invoice":
		return "Send invoice"
	case "record_payment":
		return "Record payment"
	case "close_subscription":
		return "Close subscription"
	case "pause_subscription":
		return "Pause subscription"
	case "resume_subscription":
		return "Resume subscription"
	case "renew_subscription":
		return "Renew subscription"
	default:
		return "Review proposed action"
	}
}

func approvalDetail(v Approval) string {
	inputs, _ := v.Inputs.(map[string]any)
	resourceID := v.ResourceID
	if resourceID == "" {
		resourceID = inputString(inputs, "resourceId")
	}
	switch v.Action {
	case "confirm_invoice":
		return fmt.Sprintf("Confirm and post invoice %s.", resourceID)
	case "cancel_invoice":
		return fmt.Sprintf("Cancel invoice %s.", resourceID)
	case "send_invoice":
		return fmt.Sprintf("Queue invoice %s for email delivery. Delivery status is tracked separately.", resourceID)
	case "record_payment":
		date := inputString(inputs, "date")
		if date == "" {
			date = "the approval date"
		}
		detail := fmt.Sprintf("Allocate INR %s by %s to invoice %s, received on %s.", inputString(inputs, "amount"), inputString(inputs, "method"), resourceID, date)
		if note := inputString(inputs, "note"); note != "" {
			detail += " Note: " + note
		}
		return detail
	case "close_subscription":
		return fmt.Sprintf("Close subscription %s.", resourceID)
	case "pause_subscription":
		return fmt.Sprintf("Pause subscription %s. ORCA will recheck its plan permission and status before applying the change.", resourceID)
	case "resume_subscription":
		return fmt.Sprintf("Resume subscription %s. ORCA will recheck its plan permission and status before applying the change.", resourceID)
	case "renew_subscription":
		return fmt.Sprintf("Renew subscription %s as a draft starting %s. An expiry date before the start date will be cleared.", resourceID, inputString(inputs, "startDate"))
	default:
		return fmt.Sprint(v.Inputs)
	}
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
		if req.Decision == "approve" && v.Action == "send_invoice" && !a.emailDeliveryConfigured() {
			return ErrEmailDeliveryUnavailable
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
