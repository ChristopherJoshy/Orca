package orca

import (
	"fmt"
	"strconv"
	"time"
)

// executeApprovedAction revalidates the exact stored proposal and applies its
// domain effects inside the transaction that records the approval decision.
func executeApprovedAction(st *State, approver Actor, approval Approval, now time.Time) (map[string]any, error) {
	if approval.BusinessID != approver.BusinessID || approval.ActorUserID == "" || approval.ResourceID == "" {
		return nil, ErrForbidden
	}
	proposer, ok := st.Memberships[membershipKey(approval.BusinessID, approval.ActorUserID)]
	if !ok || proposer.BusinessID != approval.BusinessID || proposer.RevokedAt != nil || (proposer.Role != "admin" && proposer.Role != "staff") {
		return nil, ErrForbidden
	}
	inputs, ok := approval.Inputs.(map[string]any)
	if !ok || len(inputs) == 0 {
		return nil, ErrConflict
	}
	hash, _, err := canonicalHash(inputs)
	if err != nil || hash != approval.InputsHash {
		return nil, ErrConflict
	}
	if inputString(inputs, "action") != approval.Action || inputString(inputs, "resourceType") != approval.ResourceType || inputString(inputs, "resourceId") != approval.ResourceID {
		return nil, ErrConflict
	}
	version, ok := inputInt64(inputs["resourceVersion"])
	if !ok || version != approval.ResourceVersion {
		return nil, ErrConflict
	}

	switch approval.Action {
	case "confirm_invoice", "cancel_invoice", "send_invoice", "record_payment":
		if approval.ResourceType != "invoice" || inputString(inputs, "invoiceId") != approval.ResourceID {
			return nil, ErrConflict
		}
		invoice, ok := st.Invoices[approval.ResourceID]
		if !ok || invoice.BusinessID != approval.BusinessID || invoice.Version != approval.ResourceVersion {
			return nil, ErrConflict
		}
		return executeApprovedInvoiceAction(st, approver, approval, inputs, invoice, now)
	case "close_subscription":
		if approval.ResourceType != "subscription" || inputString(inputs, "subscriptionId") != approval.ResourceID || !exactApprovalKeys(inputs, "action", "subscriptionId", "resourceType", "resourceId", "resourceVersion") {
			return nil, ErrConflict
		}
		sub, ok := st.Subscriptions[approval.ResourceID]
		if !ok || sub.BusinessID != approval.BusinessID || sub.Version != approval.ResourceVersion {
			return nil, ErrConflict
		}
		plan, ok := st.Plans[sub.PlanID]
		if !ok || plan.BusinessID != approval.BusinessID || !plan.AllowEarlyClose || (sub.Status != "Active" && sub.Status != "Paused" && sub.Status != "Confirmed") {
			return nil, ErrConflict
		}
		sub.Status = "Closed"
		sub.Version++
		st.Subscriptions[sub.ID] = sub
		st.Audit = append(st.Audit, auditEvent(approver, "subscription.close", "subscription", sub.ID, map[string]any{"version": sub.Version, "approvalId": approval.ID}))
		return map[string]any{"executed": true, "action": approval.Action, "resourceType": "subscription", "resourceId": sub.ID, "status": sub.Status, "version": sub.Version}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported approved action", ErrValidation)
	}
}

func executeApprovedInvoiceAction(st *State, actor Actor, approval Approval, inputs map[string]any, invoice Invoice, now time.Time) (map[string]any, error) {
	base := map[string]any{"executed": true, "action": approval.Action, "resourceType": "invoice", "resourceId": invoice.ID, "invoiceNumber": invoice.Number}
	switch approval.Action {
	case "confirm_invoice":
		if !exactApprovalKeys(inputs, "action", "invoiceId", "resourceType", "resourceId", "resourceVersion") || invoice.Status != "Draft" {
			return nil, ErrConflict
		}
		hash, _, err := canonicalHash(invoice.Snapshot)
		if err != nil || hash != invoice.CalculationHash {
			return nil, ErrConflict
		}
		invoice.Status = "Confirmed"
		invoice.Version++
		st.Invoices[invoice.ID] = invoice
		if err := postBalanced(st, actor, "invoice.confirm:"+invoice.ID, "invoice", invoice.ID, []posting{{"accounts_receivable", invoice.Total, "0.00"}, {"revenue", "0.00", subtractMoney(invoice.Total, invoice.TaxTotal)}, {"tax_payable", "0.00", invoice.TaxTotal}}); err != nil {
			return nil, err
		}
		st.Audit = append(st.Audit, auditEvent(actor, "invoice.confirmed", "invoice", invoice.ID, map[string]any{"number": invoice.Number, "total": invoice.Total, "approvalId": approval.ID}))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: invoice.BusinessID, Type: "invoice.confirmed", IdempotencyKey: "invoice.confirmed:" + invoice.ID, Payload: map[string]any{"invoiceId": invoice.ID, "approvalId": approval.ID}, CreatedAt: now})
		base["status"], base["version"] = invoice.Status, invoice.Version
	case "cancel_invoice":
		if !exactApprovalKeys(inputs, "action", "invoiceId", "resourceType", "resourceId", "resourceVersion") {
			return nil, ErrConflict
		}
		if invoice.Status == "Draft" {
			invoice.Status = "Cancelled"
		} else if invoice.Status == "Confirmed" {
			total, err := ParseDecimal(invoice.Total)
			if err != nil {
				return nil, ErrConflict
			}
			outstanding, err := ParseDecimal(invoice.Outstanding)
			if err != nil || !total.Equal(outstanding) {
				return nil, fmt.Errorf("%w: invoices with payments need a separate credit note", ErrConflict)
			}
			if err := postBalanced(st, actor, "invoice.cancel:"+invoice.ID, "invoice_reversal", invoice.ID, []posting{{"accounts_receivable", "0.00", invoice.Total}, {"revenue", subtractMoney(invoice.Total, invoice.TaxTotal), "0.00"}, {"tax_payable", invoice.TaxTotal, "0.00"}}); err != nil {
				return nil, err
			}
			invoice.Status = "Cancelled"
		} else {
			return nil, ErrConflict
		}
		invoice.Version++
		st.Invoices[invoice.ID] = invoice
		st.Audit = append(st.Audit, auditEvent(actor, "invoice.cancelled", "invoice", invoice.ID, map[string]any{"approvalId": approval.ID}))
		base["status"], base["version"] = invoice.Status, invoice.Version
	case "send_invoice":
		if !exactApprovalKeys(inputs, "action", "invoiceId", "resourceType", "resourceId", "resourceVersion") || (invoice.Status != "Confirmed" && invoice.Status != "Paid") {
			return nil, ErrConflict
		}
		st.Audit = append(st.Audit, auditEvent(actor, "invoice.email_requested", "invoice", invoice.ID, map[string]any{"approvalId": approval.ID}))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: invoice.BusinessID, Type: "invoice.email", IdempotencyKey: "invoice.email:" + invoice.ID + ":approval:" + approval.ID, Payload: map[string]any{"invoiceId": invoice.ID, "approvalId": approval.ID}, CreatedAt: now})
		base["queued"], base["status"] = true, invoice.Status
	case "record_payment":
		if !exactApprovalKeys(inputs, "action", "invoiceId", "amount", "method", "date", "note", "operationKey", "resourceType", "resourceId", "resourceVersion") || invoice.Status != "Confirmed" {
			return nil, ErrConflict
		}
		amountText := inputString(inputs, "amount")
		amount, err := ParseDecimal(amountText)
		if err != nil || !amount.IsPositive() || amount.StringFixed(2) != amountText {
			return nil, ErrConflict
		}
		method, note, operationKey, date := inputString(inputs, "method"), inputString(inputs, "note"), inputString(inputs, "operationKey"), inputString(inputs, "date")
		if method == "" || len(method) > 40 || len(note) > 500 || operationKey == "" || len(operationKey) > 120 {
			return nil, ErrConflict
		}
		receivedAt := now
		if date != "" {
			if len(date) == 10 {
				receivedAt, err = time.Parse("2006-01-02", date)
			} else {
				receivedAt, err = time.Parse(time.RFC3339, date)
			}
			if err != nil {
				return nil, ErrConflict
			}
			receivedAt = receivedAt.UTC()
		}
		for _, existing := range st.Payments {
			if existing.OperationKey == operationKey {
				return nil, ErrConflict
			}
		}
		outstanding, err := ParseDecimal(invoice.Outstanding)
		if err != nil || !outstanding.IsPositive() || amount.GreaterThan(outstanding) {
			return nil, ErrConflict
		}
		paymentID, err := newUUID()
		if err != nil {
			return nil, err
		}
		payment := Payment{ID: paymentID, BusinessID: invoice.BusinessID, CustomerID: invoice.CustomerID, InvoiceID: invoice.ID, Amount: amount.StringFixed(2), Method: method, Note: note, Provider: "manual", OperationKey: operationKey, ReceivedAt: receivedAt, CreatedAt: now}
		st.Payments[paymentID] = payment
		st.Allocations[paymentID+":"+invoice.ID] = PaymentAllocation{BusinessID: invoice.BusinessID, PaymentID: paymentID, InvoiceID: invoice.ID, Amount: payment.Amount, CreatedAt: now}
		outstanding = outstanding.Sub(amount)
		invoice.Outstanding = amountString(maxZero(outstanding))
		if invoice.Outstanding == "0.00" {
			invoice.Status = "Paid"
		}
		invoice.Version++
		st.Invoices[invoice.ID] = invoice
		if err := postBalanced(st, actor, "payment:"+operationKey, "payment", paymentID, []posting{{"cash", payment.Amount, "0.00"}, {"accounts_receivable", "0.00", payment.Amount}}); err != nil {
			return nil, err
		}
		st.Audit = append(st.Audit, auditEvent(actor, "payment.allocated", "payment", paymentID, map[string]any{"invoiceId": invoice.ID, "amount": payment.Amount, "approvalId": approval.ID}))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: invoice.BusinessID, Type: "payment.recorded", IdempotencyKey: "payment.recorded:" + operationKey, Payload: map[string]any{"paymentId": paymentID, "invoiceId": invoice.ID, "approvalId": approval.ID}, CreatedAt: now})
		base["paymentId"], base["amount"], base["status"], base["outstanding"] = paymentID, payment.Amount, invoice.Status, invoice.Outstanding
		base["customerId"], base["method"], base["note"], base["receivedAt"] = payment.CustomerID, payment.Method, payment.Note, payment.ReceivedAt
	}
	return base, nil
}

func inputString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func inputInt64(value any) (int64, bool) {
	switch n := value.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if n != float64(int64(n)) {
			return 0, false
		}
		return int64(n), true
	case string:
		parsed, err := strconv.ParseInt(n, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func exactApprovalKeys(values map[string]any, expected ...string) bool {
	if len(values) != len(expected) {
		return false
	}
	for _, key := range expected {
		if _, ok := values[key]; !ok {
			return false
		}
	}
	return true
}
