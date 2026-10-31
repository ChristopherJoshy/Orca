package orca

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

type createSubscriptionRequest struct {
	CustomerID       string
	PlanID           string
	DiscountID       string
	StartDate        string
	ExpiryDate       string
	PaymentTermsDays int
	Lines            []SubscriptionLine
	QuotationID      string
}

func (a *API) createSubscription(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := checkActorStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	var req createSubscriptionRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.PaymentTermsDays < 0 || req.PaymentTermsDays > 365 {
		writeErr(w, fmt.Errorf("%w: payment terms must be from 0 to 365 days", ErrValidation))
		return
	}
	id, err := newUUID()
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	var created Subscription
	var responsePlan Plan
	err = a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		if req.QuotationID != "" {
			quote, ok := st.Quotations[req.QuotationID]
			if !ok {
				return ErrNotFound
			}
			if quote.Status != "Accepted" {
				return fmt.Errorf("%w: quotation must be accepted before conversion", ErrConflict)
			}
			if req.CustomerID == "" {
				req.CustomerID = quote.CustomerID
			}
			if req.PlanID == "" {
				req.PlanID = quote.PlanID
			}
			if len(req.Lines) == 0 {
				req.Lines = append([]SubscriptionLine(nil), quote.Lines...)
			}
		}
		customer, ok := st.Customers[req.CustomerID]
		if !ok {
			return ErrNotFound
		}
		plan, ok := st.Plans[req.PlanID]
		if !ok {
			return ErrNotFound
		}
		responsePlan = plan
		start := req.StartDate
		if start == "" {
			location, err := time.LoadLocation(st.Businesses[auth.Actor.BusinessID].Timezone)
			if err != nil {
				location = time.UTC
			}
			start = now.In(location).Format("2006-01-02")
		}
		if _, err := time.Parse("2006-01-02", start); err != nil {
			return fmt.Errorf("%w: startDate must use YYYY-MM-DD", ErrValidation)
		}
		if plan.AvailableFrom != "" && start < plan.AvailableFrom || plan.AvailableUntil != "" && start > plan.AvailableUntil {
			return fmt.Errorf("%w: subscription start date is outside plan availability", ErrValidation)
		}
		if req.ExpiryDate != "" {
			if _, err := time.Parse("2006-01-02", req.ExpiryDate); err != nil {
				return fmt.Errorf("%w: expiryDate must use YYYY-MM-DD", ErrValidation)
			}
		}
		lines := req.Lines
		if len(lines) == 0 {
			lines = []SubscriptionLine{{Name: plan.Name, Quantity: plan.MinimumQuantity, UnitPrice: plan.SalesPrice, TaxRate: plan.TaxRate}}
		}
		for i := range lines {
			if lines[i].Quantity < 1 {
				return fmt.Errorf("%w: quantity must be positive", ErrValidation)
			}
			if lines[i].UnitPrice == "" {
				lines[i].UnitPrice = plan.SalesPrice
			}
			if lines[i].TaxRate == "" {
				lines[i].TaxRate = plan.TaxRate
			}
			if err := validatePrice(lines[i].UnitPrice); err != nil {
				return err
			}
			if err := validateRate(lines[i].TaxRate); err != nil {
				return err
			}
			if lines[i].Name == "" {
				lines[i].Name = plan.Name
			}
			if lines[i].ProductID != "" {
				if p, found := st.Products[lines[i].ProductID]; !found {
					return ErrNotFound
				} else if lines[i].TaxRate == "0" {
					lines[i].TaxRate = p.TaxRate
				}
			}
		}
		if len(lines) == 0 || lines[0].Quantity < plan.MinimumQuantity {
			return fmt.Errorf("%w: quantity is below the plan minimum", ErrValidation)
		}
		if req.DiscountID != "" {
			if _, ok := st.Discounts[req.DiscountID]; !ok {
				return ErrNotFound
			}
		}
		number := fmt.Sprintf("SUB-%04d", len(st.Subscriptions)+1)
		created = Subscription{ID: id, BusinessID: auth.Actor.BusinessID, CustomerID: customer.ID, PlanID: plan.ID, DiscountID: req.DiscountID, Number: number, Status: "Draft", StartDate: start, ExpiryDate: req.ExpiryDate, PaymentTermsDays: req.PaymentTermsDays, Lines: lines, Version: 1, CreatedAt: now}
		st.Subscriptions[id] = created
		if req.QuotationID != "" {
			quote := st.Quotations[req.QuotationID]
			quote.Status = "Converted"
			st.Quotations[req.QuotationID] = quote
		}
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "subscription.created", "subscription", id, map[string]any{"number": number}))
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	state := NewState()
	state.Plans[responsePlan.ID] = responsePlan
	writeJSON(w, 201, subscriptionWire(created, state))
}

func (a *API) transitionSubscription(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	id := chi.URLParam(r, "id")
	var req struct {
		Action          string
		ExpectedVersion int64
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	action := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(req.Action), " ", "_"))
	var result Subscription
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		v, ok := st.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		if auth.Actor.Role == "customer" {
			if v.CustomerID != auth.Actor.CustomerID {
				return ErrNotFound
			}
			if action != "pause" && action != "resume" {
				return ErrForbidden
			}
		} else if err := requireStaff(auth); err != nil {
			return err
		}
		if req.ExpectedVersion > 0 && req.ExpectedVersion != v.Version {
			return ErrConflict
		}
		plan := st.Plans[v.PlanID]
		now := time.Now().UTC()
		switch action {
		case "request_quote", "quote":
			if v.Status != "Draft" {
				return ErrConflict
			}
			v.Status = "Quotation"
		case "confirm":
			if v.Status != "Quotation" {
				return ErrConflict
			}
			v.Status = "Confirmed"
		case "activate":
			if v.Status != "Confirmed" {
				return ErrConflict
			}
			start, err := ParseLocalDate(v.StartDate, st.Businesses[v.BusinessID].Timezone)
			if err != nil {
				return err
			}
			if !start.After(now) {
				v.Status = "Active"
				v.BillingCursor = &start
			}
		case "pause":
			if v.Status != "Active" || !plan.AllowPause {
				return ErrConflict
			}
			v.Status = "Paused"
			v.PauseIntervals = append(v.PauseIntervals, PauseInterval{Start: now})
		case "resume":
			if v.Status != "Paused" || !plan.AllowPause {
				return ErrConflict
			}
			v.Status = "Active"
			if len(v.PauseIntervals) > 0 {
				end := now
				v.PauseIntervals[len(v.PauseIntervals)-1].End = &end
			}
		case "close":
			if (v.Status != "Active" && v.Status != "Paused" && v.Status != "Confirmed") || !plan.AllowEarlyClose {
				return ErrConflict
			}
			v.Status = "Closed"
		case "renew":
			if v.Status != "Closed" || !plan.AllowRenew {
				return ErrConflict
			}
			v.Status = "Draft"
			v.StartDate = now.Format("2006-01-02")
			v.BillingCursor = nil
		default:
			return fmt.Errorf("%w: unsupported subscription action", ErrValidation)
		}
		v.Version++
		st.Subscriptions[id] = v
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "subscription."+action, "subscription", id, map[string]any{"version": v.Version}))
		result = v
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (a *API) runBilling(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	cutoff := time.Now().UTC()
	count := 0
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		business := st.Businesses[auth.Actor.BusinessID]
		loc, e := time.LoadLocation(business.Timezone)
		if e != nil {
			loc = time.UTC
		}
		ids := make([]string, 0, len(st.Subscriptions))
		for id := range st.Subscriptions {
			ids = append(ids, id)
		}
		for _, id := range ids {
			v := st.Subscriptions[id]
			if v.Status == "Confirmed" {
				start, e := ParseLocalDate(v.StartDate, business.Timezone)
				if e == nil && !start.After(cutoff) {
					v.Status = "Active"
					v.BillingCursor = &start
					v.Version++
					st.Subscriptions[id] = v
				}
			}
			if v.Status != "Active" {
				continue
			}
			plan, ok := st.Plans[v.PlanID]
			if !ok {
				continue
			}
			if v.ExpiryDate != "" {
				if expires, e := ParseLocalDate(v.ExpiryDate, business.Timezone); e == nil && cutoff.After(expires) {
					if plan.AutoClose {
						v.Status = "Closed"
						v.Version++
						st.Subscriptions[id] = v
					}
					continue
				}
			}
			cursor := v.BillingCursor
			if cursor == nil {
				start, e := ParseLocalDate(v.StartDate, business.Timezone)
				if e != nil {
					return e
				}
				cursor = &start
			}
			anchorTime, e := time.ParseInLocation("2006-01-02", v.StartDate, loc)
			if e != nil {
				return e
			}
			anchor := AnchorFor(anchorTime, plan.Interval, loc)
			for iterations := 0; iterations < 1200 && !cursor.After(cutoff); iterations++ {
				start := cursor.In(loc)
				end, e := NextPeriod(start, plan.Interval, anchor, loc)
				if e != nil {
					return e
				}
				already := false
				for _, old := range st.Invoices {
					if old.SubscriptionID == v.ID && old.PeriodStart.Equal(start.UTC()) {
						already = true
						break
					}
				}
				if !already {
					inputs := make([]CalculationInputLine, 0, len(v.Lines))
					for _, line := range v.Lines {
						product := Product{ID: line.ProductID, Version: plan.Version}
						if p, found := st.Products[line.ProductID]; found {
							product = p
						}
						taxID := ""
						taxVersion := int64(0)
						lineRate, _ := ParseDecimal(line.TaxRate)
						for _, tax := range st.Taxes {
							rate, parseErr := ParseDecimal(tax.Rate)
							if tax.Active && parseErr == nil && rate.Equal(lineRate) && (taxID == "" || tax.ID < taxID) {
								taxID, taxVersion = tax.ID, tax.Version
							}
						}
						inputs = append(inputs, CalculationInputLine{Product: product, Quantity: line.Quantity, UnitPrice: line.UnitPrice, TaxRate: line.TaxRate, Description: line.Name, Variant: line.Variant, PlanID: plan.ID, PlanVersion: plan.Version, TaxID: taxID, TaxVersion: taxVersion})
					}
					var discount *Discount
					if v.DiscountID != "" {
						d, found := st.Discounts[v.DiscountID]
						if found {
							discount = &d
						}
					}
					snapshot, hash, e := Calculate(inputs, discount, business.ID, v.ID, start.UTC(), end, business.Timezone)
					if e != nil {
						return e
					}
					invoiceID, e := newUUID()
					if e != nil {
						return e
					}
					number := fmt.Sprintf("INV-%04d-%05d", start.Year(), business.NextInvoiceNumber)
					business.NextInvoiceNumber++
					due := end.In(loc).AddDate(0, 0, v.PaymentTermsDays).UTC()
					inv := Invoice{ID: invoiceID, BusinessID: business.ID, SubscriptionID: v.ID, CustomerID: v.CustomerID, Number: number, PeriodStart: start.UTC(), PeriodEnd: end, Status: "Draft", Currency: CurrencyINR, Subtotal: snapshot.Subtotal, DiscountTotal: snapshot.DiscountTotal, TaxTotal: snapshot.TaxTotal, Total: snapshot.Total, Outstanding: snapshot.Total, DueAt: due, Snapshot: snapshot, CalculationHash: hash, Version: 1, CreatedAt: cutoff}
					st.Invoices[inv.ID] = inv
					if discount != nil && snapshot.DiscountID != "" {
						d := st.Discounts[discount.ID]
						d.UseCount++
						st.Discounts[discount.ID] = d
					}
					st.Audit = append(st.Audit, auditEvent(auth.Actor, "invoice.generated", "invoice", inv.ID, map[string]any{"periodStart": start.UTC(), "periodEnd": end}))
					st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: business.ID, Type: "invoice.generated", IdempotencyKey: "invoice.generated:" + inv.ID, Payload: map[string]any{"invoiceId": inv.ID}, CreatedAt: cutoff})
					count++
				}
				endUTC := end.UTC()
				v.BillingCursor = &endUTC
				v.Version++
				st.Subscriptions[id] = v
				cursor = &endUTC
				if cursor.After(cutoff) {
					break
				}
			}
		}
		st.Businesses[business.ID] = business
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"created": count, "asOf": cutoff})
}
func mustNewUUID() string {
	id, err := newUUID()
	if err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	return id
}

func (a *API) getInvoice(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	id := chi.URLParam(r, "id")
	var v Invoice
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		v, ok = st.Invoices[id]
		if !ok || (auth.Actor.Role == "customer" && v.CustomerID != auth.Actor.CustomerID) {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, invoiceWire(v))
}
func (a *API) confirmInvoice(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var result Invoice
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		v, ok := st.Invoices[id]
		if !ok {
			return ErrNotFound
		}
		if v.Status != "Draft" {
			return ErrConflict
		}
		v.Status = "Confirmed"
		v.Version++
		st.Invoices[id] = v
		if err := postBalanced(st, auth.Actor, "invoice.confirm:"+id, "invoice", id, []posting{{"accounts_receivable", v.Total, "0.00"}, {"revenue", "0.00", subtractMoney(v.Total, v.TaxTotal)}, {"tax_payable", "0.00", v.TaxTotal}}); err != nil {
			return err
		}
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "invoice.confirmed", "invoice", id, map[string]any{"number": v.Number, "total": v.Total}))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: v.BusinessID, Type: "invoice.confirmed", IdempotencyKey: "invoice.confirmed:" + id, Payload: map[string]any{"invoiceId": id}, CreatedAt: time.Now().UTC()})
		result = v
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, invoiceWire(result))
}

type posting struct{ account, debit, credit string }

func postBalanced(st *State, actor Actor, op, sourceType, sourceID string, all []posting) error {
	debitTotal, creditTotal := decimal.Zero, decimal.Zero
	for _, p := range all {
		d, err := ParseDecimal(p.debit)
		if err != nil {
			return err
		}
		c, err := ParseDecimal(p.credit)
		if err != nil {
			return err
		}
		if d.IsNegative() || c.IsNegative() || d.IsPositive() && c.IsPositive() {
			return errors.New("invalid journal posting")
		}
		debitTotal = debitTotal.Add(d)
		creditTotal = creditTotal.Add(c)
	}
	if !debitTotal.Equal(creditTotal) {
		return errors.New("journal is not balanced")
	}
	now := time.Now().UTC()
	for _, p := range all {
		d, _ := ParseDecimal(p.debit)
		c, _ := ParseDecimal(p.credit)
		if d.IsZero() && c.IsZero() {
			continue
		}
		id, err := newUUID()
		if err != nil {
			return err
		}
		st.Journal = append(st.Journal, JournalEntry{ID: id, BusinessID: actor.BusinessID, OperationKey: op, SourceType: sourceType, SourceID: sourceID, Account: p.account, Debit: money2(p.debit), Credit: money2(p.credit), Currency: CurrencyINR, CreatedAt: now})
	}
	return nil
}
func (a *API) cancelInvoice(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var result Invoice
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		v, ok := st.Invoices[id]
		if !ok {
			return ErrNotFound
		}
		if v.Status == "Cancelled" {
			result = v
			return nil
		}
		if v.Status == "Draft" {
			v.Status = "Cancelled"
		} else if v.Status == "Confirmed" {
			total, _ := ParseDecimal(v.Total)
			open, _ := ParseDecimal(v.Outstanding)
			if !total.Equal(open) {
				return fmt.Errorf("%w: invoices with payments need a separate credit note", ErrConflict)
			}
			if err := postBalanced(st, auth.Actor, "invoice.cancel:"+id, "invoice_reversal", id, []posting{{"accounts_receivable", "0.00", v.Total}, {"revenue", subtractMoney(v.Total, v.TaxTotal), "0.00"}, {"tax_payable", v.TaxTotal, "0.00"}}); err != nil {
				return err
			}
			v.Status = "Cancelled"
		} else {
			return ErrConflict
		}
		v.Version++
		st.Invoices[id] = v
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "invoice.cancelled", "invoice", id, nil))
		result = v
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, invoiceWire(result))
}
func (a *API) sendInvoice(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if err := requireStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var v Invoice
	err := a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		var ok bool
		v, ok = st.Invoices[id]
		if !ok {
			return ErrNotFound
		}
		if v.Status != "Confirmed" && v.Status != "Paid" {
			return ErrConflict
		}
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "invoice.email_requested", "invoice", id, nil))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: v.BusinessID, Type: "invoice.email", IdempotencyKey: "invoice.email:" + id, Payload: map[string]any{"invoiceId": id}, CreatedAt: time.Now().UTC()})
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 202, map[string]any{"queued": true, "invoiceId": v.ID})
}
func (a *API) recordPayment(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role == "customer" {
		writeErr(w, ErrForbidden)
		return
	}
	if err := requireStaff(auth); err != nil {
		writeErr(w, err)
		return
	}
	invoiceID := chi.URLParam(r, "id")
	var req struct {
		Amount       string
		Method       string
		Note         string
		Date         string
		ReceivedAt   string
		OperationKey string
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.OperationKey == "" {
		req.OperationKey = r.Header.Get("Idempotency-Key")
	}
	if req.OperationKey == "" {
		writeErr(w, fmt.Errorf("%w: Idempotency-Key is required", ErrValidation))
		return
	}
	amount, err := ParseDecimal(req.Amount)
	if err != nil || !amount.IsPositive() {
		writeErr(w, ErrInvalidMoney)
		return
	}
	received := time.Now().UTC()
	date := req.Date
	if date == "" {
		date = req.ReceivedAt
	}
	if date != "" {
		if len(date) == 10 {
			parsed, e := time.Parse("2006-01-02", date)
			if e != nil {
				writeErr(w, fmt.Errorf("%w: invalid payment date", ErrValidation))
				return
			}
			received = parsed.UTC()
		} else if parsed, e := time.Parse(time.RFC3339, date); e == nil {
			received = parsed.UTC()
		} else {
			writeErr(w, fmt.Errorf("%w: invalid payment date", ErrValidation))
			return
		}
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = "Manual"
	}
	var result Payment
	err = a.Store.UpdateFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		inv, ok := st.Invoices[invoiceID]
		if !ok {
			return ErrNotFound
		}
		for _, old := range st.Payments {
			if old.OperationKey != req.OperationKey {
				continue
			}
			if old.InvoiceID != invoiceID || old.Amount != amountString(amount) || old.Method != method || old.Note != req.Note || date != "" && !old.ReceivedAt.Equal(received) {
				return ErrConflict
			}
			result = old
			return nil
		}
		if inv.Status != "Confirmed" {
			return ErrConflict
		}
		out, _ := ParseDecimal(inv.Outstanding)
		if amount.GreaterThan(out) {
			return fmt.Errorf("%w: payment exceeds outstanding balance", ErrValidation)
		}
		id, e := newUUID()
		if e != nil {
			return e
		}
		result = Payment{ID: id, BusinessID: inv.BusinessID, CustomerID: inv.CustomerID, InvoiceID: inv.ID, Amount: amountString(amount), Method: method, Note: req.Note, Provider: "manual", OperationKey: req.OperationKey, ReceivedAt: received, CreatedAt: time.Now().UTC()}
		st.Payments[id] = result
		allocation := PaymentAllocation{BusinessID: inv.BusinessID, PaymentID: id, InvoiceID: inv.ID, Amount: result.Amount, CreatedAt: result.CreatedAt}
		st.Allocations[id+":"+inv.ID] = allocation
		paid := decimal.Zero
		for _, al := range st.Allocations {
			if al.InvoiceID == inv.ID {
				d, _ := ParseDecimal(al.Amount)
				paid = paid.Add(d)
			}
		}
		total, _ := ParseDecimal(inv.Total)
		inv.Outstanding = amountString(maxZero(total.Sub(paid)))
		if inv.Outstanding == "0.00" {
			inv.Status = "Paid"
		}
		inv.Version++
		st.Invoices[inv.ID] = inv
		if e := postBalanced(st, auth.Actor, "payment:"+req.OperationKey, "payment", id, []posting{{"cash", result.Amount, "0.00"}, {"accounts_receivable", "0.00", result.Amount}}); e != nil {
			return e
		}
		st.Audit = append(st.Audit, auditEvent(auth.Actor, "payment.allocated", "payment", id, map[string]any{"invoiceId": inv.ID, "amount": result.Amount}))
		st.Outbox = append(st.Outbox, OutboxEvent{ID: mustNewUUID(), BusinessID: inv.BusinessID, Type: "payment.recorded", IdempotencyKey: "payment.recorded:" + req.OperationKey, Payload: map[string]any{"paymentId": id, "invoiceId": inv.ID}, CreatedAt: time.Now().UTC()})
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, paymentWire(result))
}
func (a *API) reportSummary(w http.ResponseWriter, r *http.Request, auth Authenticated) {
	if auth.Actor.Role == "customer" {
		writeErr(w, ErrForbidden)
		return
	}
	var result map[string]any
	err := a.Store.ReadFor(r.Context(), auth.Actor.BusinessID, func(st *State) error {
		active := 0
		recurring := decimal.Zero
		invoiced := decimal.Zero
		receivable := decimal.Zero
		overdue := 0
		collected := decimal.Zero
		now := time.Now().UTC()
		for _, s := range st.Subscriptions {
			if s.Status == "Active" {
				active++
				p := st.Plans[s.PlanID]
				q := 1
				if len(s.Lines) > 0 {
					q = s.Lines[0].Quantity
				}
				d, e := ParseDecimal(p.SalesPrice)
				if e == nil {
					recurring = recurring.Add(d.Mul(decimal.NewFromInt(int64(q))))
				}
			}
		}
		for _, v := range st.Invoices {
			total, _ := ParseDecimal(v.Total)
			out, _ := ParseDecimal(v.Outstanding)
			invoiced = invoiced.Add(total)
			receivable = receivable.Add(out)
			if v.Status == "Confirmed" && out.IsPositive() && now.After(v.DueAt) {
				overdue++
			}
		}
		for _, v := range st.Payments {
			d, e := ParseDecimal(v.Amount)
			if e == nil {
				collected = collected.Add(d)
			}
		}
		result = map[string]any{"asOf": now, "currency": CurrencyINR, "activeSubscriptions": active, "recurringRevenue": amountString(recurring), "invoiced": amountString(invoiced), "receivables": amountString(receivable), "overdueInvoices": overdue, "collected": amountString(collected), "integrity": map[string]any{"journalBalanced": journalBalanced(st.Journal), "invoiceHashMismatches": invoiceHashMismatches(st.Invoices), "outboxPending": pendingOutbox(st.Outbox)}}
		return nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func journalBalanced(list []JournalEntry) bool {
	d, c := decimal.Zero, decimal.Zero
	for _, v := range list {
		dv, e1 := ParseDecimal(v.Debit)
		cv, e2 := ParseDecimal(v.Credit)
		if e1 == nil {
			d = d.Add(dv)
		}
		if e2 == nil {
			c = c.Add(cv)
		}
	}
	return d.Equal(c)
}
func invoiceHashMismatches(invoices map[string]Invoice) int {
	bad := 0
	for _, v := range invoices {
		hash, _, err := canonicalHash(v.Snapshot)
		if err != nil || hash != v.CalculationHash {
			bad++
		}
	}
	return bad
}
func pendingOutbox(events []OutboxEvent) int {
	n := 0
	for _, v := range events {
		if v.DeliveredAt == nil {
			n++
		}
	}
	return n
}
