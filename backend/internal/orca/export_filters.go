package orca

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// exportFilter keeps the accepted, versionable filter vocabulary separate
// from raw JSON request values. Unknown keys fail the export instead of
// silently producing a full archive after a typo.
type exportFilter struct {
	resource                  string
	ids                       map[string]struct{}
	hasIDs                    bool
	status, search            string
	customerID, invoiceID     string
	subscriptionID, productID string
	planID, discountID        string
	from, to                  time.Time
	hasFrom, hasTo            bool
}

var allowedExportFilterKeys = map[string]struct{}{
	"kind": {}, "resource": {}, "dataset": {}, "id": {}, "ids": {},
	"status": {}, "search": {}, "customerId": {}, "invoiceId": {},
	"subscriptionId": {}, "productId": {}, "planId": {}, "discountId": {},
	"dateFrom": {}, "dateTo": {}, "from": {}, "to": {},
}

func parseExportFilter(raw map[string]any) (exportFilter, bool, error) {
	f := exportFilter{ids: map[string]struct{}{}}
	if len(raw) == 0 {
		return f, false, nil
	}
	for key := range raw {
		if _, ok := allowedExportFilterKeys[key]; !ok {
			return f, false, fmt.Errorf("%w: unsupported export filter %q", ErrValidation, key)
		}
	}
	for _, key := range []string{"kind", "resource", "dataset", "id", "status", "search", "customerId", "invoiceId", "subscriptionId", "productId", "planId", "discountId"} {
		if value, ok := raw[key]; ok {
			if _, valid := value.(string); !valid {
				return f, false, fmt.Errorf("%w: export filter %q must be a string", ErrValidation, key)
			}
		}
	}
	kind := strings.ToLower(exportFilterString(raw["kind"]))
	if kind != "" && kind != "full" && kind != "filtered" {
		return f, false, fmt.Errorf("%w: export kind must be full or filtered", ErrValidation)
	}
	if kind == "full" && len(raw) == 1 {
		return f, false, nil
	}
	requestedResource := strings.ToLower(strings.TrimSpace(exportFilterString(raw["resource"])))
	requestedDataset := strings.ToLower(strings.TrimSpace(exportFilterString(raw["dataset"])))
	if requestedResource != "" && requestedDataset != "" && canonicalExportResource(requestedResource) != canonicalExportResource(requestedDataset) {
		return f, false, fmt.Errorf("%w: resource and dataset filters must match", ErrValidation)
	}
	requested := requestedResource
	if requested == "" {
		requested = requestedDataset
	}
	f.resource = canonicalExportResource(requested)
	if f.resource == "" {
		if requested != "" {
			return f, false, fmt.Errorf("%w: unsupported export resource %q", ErrValidation, requested)
		}
		if kind == "filtered" || len(raw) > 1 {
			return f, false, fmt.Errorf("%w: a resource is required for a filtered export", ErrValidation)
		}
		return f, false, nil
	}
	f.status = strings.TrimSpace(exportFilterString(raw["status"]))
	f.search = strings.TrimSpace(exportFilterString(raw["search"]))
	if len(f.search) > 200 {
		return f, false, fmt.Errorf("%w: export search is too long", ErrValidation)
	}
	f.customerID = strings.TrimSpace(exportFilterString(raw["customerId"]))
	f.invoiceID = strings.TrimSpace(exportFilterString(raw["invoiceId"]))
	f.subscriptionID = strings.TrimSpace(exportFilterString(raw["subscriptionId"]))
	f.productID = strings.TrimSpace(exportFilterString(raw["productId"]))
	f.planID = strings.TrimSpace(exportFilterString(raw["planId"]))
	f.discountID = strings.TrimSpace(exportFilterString(raw["discountId"]))
	if id := strings.TrimSpace(exportFilterString(raw["id"])); id != "" {
		f.hasIDs = true
		f.ids[id] = struct{}{}
	} else if _, ok := raw["id"]; ok {
		return f, false, fmt.Errorf("%w: export id must be a non-empty string", ErrValidation)
	}
	if values, ok := raw["ids"]; ok {
		f.hasIDs = true
		list, valid := values.([]any)
		if !valid {
			if stringsList, isStrings := values.([]string); isStrings {
				list = make([]any, len(stringsList))
				for i := range stringsList {
					list[i] = stringsList[i]
				}
			} else {
				return f, false, fmt.Errorf("%w: export ids must be an array of strings", ErrValidation)
			}
		}
		if len(list) > 5000 {
			return f, false, fmt.Errorf("%w: no more than 5000 ids may be exported at once", ErrValidation)
		}
		for _, value := range list {
			id, ok := value.(string)
			if !ok || strings.TrimSpace(id) == "" {
				return f, false, fmt.Errorf("%w: export ids must be non-empty strings", ErrValidation)
			}
			f.ids[strings.TrimSpace(id)] = struct{}{}
		}
	}
	if f.status != "" && !exportResourceHasStatus(f.resource) {
		return f, false, fmt.Errorf("%w: status is not supported for export resource %q", ErrValidation, f.resource)
	}
	if err := validateExportSelectors(f); err != nil {
		return f, false, err
	}
	if rawFrom, ok := firstExportValue(raw, "dateFrom", "from"); ok {
		parsed, err := parseExportDate(rawFrom)
		if err != nil {
			return f, false, err
		}
		f.from, f.hasFrom = parsed, true
	}
	if rawTo, ok := firstExportValue(raw, "dateTo", "to"); ok {
		parsed, err := parseExportDate(rawTo)
		if err != nil {
			return f, false, err
		}
		f.to, f.hasTo = parsed, true
	}
	if f.hasFrom && f.hasTo && f.to.Before(f.from) {
		return f, false, fmt.Errorf("%w: export dateTo must not be before dateFrom", ErrValidation)
	}
	return f, true, nil
}

func validateExportSelectors(f exportFilter) error {
	allowed := map[string]bool{}
	switch f.resource {
	case "customers":
		allowed["customer"] = true
	case "products":
		allowed["product"] = true
	case "plans":
		allowed["plan"] = true
	case "discounts":
		allowed["discount"] = true
	case "quotation_templates":
		allowed["product"] = true
	case "quotations":
		allowed["customer"], allowed["product"], allowed["plan"] = true, true, true
	case "subscriptions":
		allowed["customer"], allowed["subscription"], allowed["product"], allowed["plan"], allowed["discount"] = true, true, true, true, true
	case "invoices":
		allowed["customer"], allowed["invoice"], allowed["subscription"], allowed["product"], allowed["plan"], allowed["discount"] = true, true, true, true, true, true
	case "payments":
		allowed["customer"], allowed["invoice"], allowed["subscription"], allowed["product"], allowed["plan"], allowed["discount"] = true, true, true, true, true, true
	case "journal":
		allowed["invoice"], allowed["subscription"] = true, true
	}
	for name, value := range map[string]string{"customerId": f.customerID, "invoiceId": f.invoiceID, "subscriptionId": f.subscriptionID, "productId": f.productID, "planId": f.planID, "discountId": f.discountID} {
		if value != "" {
			kind := strings.TrimSuffix(name, "Id")
			if !allowed[kind] {
				return fmt.Errorf("%w: filter %q is not supported for export resource %q", ErrValidation, name, f.resource)
			}
		}
	}
	return nil
}

func canonicalExportResource(value string) string {
	switch value {
	case "tax", "taxes":
		return "tax_rules"
	case "quote_templates", "quotation_templates":
		return "quotation_templates"
	case "customer":
		return "customers"
	case "customers", "products", "plans", "discounts", "quotations", "subscriptions", "invoices", "payments", "journal", "audit", "approvals":
		return value
	case "product":
		return "products"
	case "plan":
		return "plans"
	case "discount":
		return "discounts"
	case "quotation":
		return "quotations"
	case "subscription":
		return "subscriptions"
	case "invoice":
		return "invoices"
	case "payment":
		return "payments"
	case "tax_rules":
		return "tax_rules"
	default:
		return ""
	}
}

func exportResourceHasStatus(resource string) bool {
	switch resource {
	case "quotations", "subscriptions", "invoices", "approvals":
		return true
	default:
		return false
	}
}

func exportFilterString(value any) string {
	s, _ := value.(string)
	return s
}

func firstExportValue(values map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func parseExportDate(value any) (time.Time, error) {
	text, ok := value.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("%w: export dates must use YYYY-MM-DD", ErrValidation)
	}
	date, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: export dates must use YYYY-MM-DD", ErrValidation)
	}
	return date.UTC(), nil
}

func (f exportFilter) matchesID(id string) bool {
	if !f.hasIDs {
		return true
	}
	_, ok := f.ids[id]
	return ok
}

func (f exportFilter) matchesStatus(status string) bool {
	return f.status == "" || strings.EqualFold(strings.TrimSpace(status), f.status)
}

func (f exportFilter) matchesSearch(fields ...string) bool {
	if f.search == "" {
		return true
	}
	want := strings.ToLower(f.search)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), want) {
			return true
		}
	}
	return false
}

func (f exportFilter) matchesDate(date time.Time) bool {
	if !f.hasFrom && !f.hasTo {
		return true
	}
	if date.IsZero() {
		return false
	}
	day := time.Date(date.UTC().Year(), date.UTC().Month(), date.UTC().Day(), 0, 0, 0, 0, time.UTC)
	if f.hasFrom && day.Before(f.from) {
		return false
	}
	return !f.hasTo || !day.After(f.to)
}

func matchesAnyProduct(lines []SubscriptionLine, productID string) bool {
	if productID == "" {
		return true
	}
	for _, line := range lines {
		if line.ProductID == productID {
			return true
		}
	}
	return false
}

func matchesInvoiceProduct(invoice Invoice, productID string) bool {
	if productID == "" {
		return true
	}
	for _, line := range invoice.Snapshot.Lines {
		if line.ProductID == productID {
			return true
		}
	}
	return false
}

func matchesInvoiceTax(invoice Invoice, taxID string) bool {
	if taxID == "" {
		return true
	}
	for _, line := range invoice.Snapshot.Lines {
		if line.TaxID == taxID {
			return true
		}
	}
	return false
}

func matchesInvoiceID(invoice Invoice, invoiceID string) bool {
	return invoiceID == "" || invoice.ID == invoiceID
}

func matchesInvoiceSubscription(invoice Invoice, subscriptionID string) bool {
	return subscriptionID == "" || invoice.SubscriptionID == subscriptionID
}

func matchesInvoiceDiscount(invoice Invoice, discountID string) bool {
	return discountID == "" || invoice.Snapshot.DiscountID == discountID
}

func matchesInvoicePlan(invoice Invoice, planID string) bool {
	return planID == "" || invoice.Snapshot.PlanID == planID
}

// filterExportState always starts from a business-scoped copy, then applies a
// resource filter and retains the records referenced by the selected rows.
// Sensitive identity and runtime state is deliberately never copied.
func filterExportState(state *State, businessID string, raw map[string]any) (*State, error) {
	scoped := businessExportState(state, businessID)
	f, apply, err := parseExportFilter(raw)
	if err != nil {
		return nil, err
	}
	if !apply {
		return scoped, nil
	}
	out := NewState()
	out.Businesses = scoped.Businesses

	switch f.resource {
	case "customers":
		for id, v := range scoped.Customers {
			if f.matchesID(v.ID) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Name, v.Email, v.Company, v.Phone) && (f.customerID == "" || f.customerID == v.ID) {
				out.Customers[id] = v
			}
		}
	case "products":
		for id, v := range scoped.Products {
			if f.matchesID(v.ID) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Name, v.Type, v.Notes) && (f.productID == "" || f.productID == v.ID) {
				out.Products[id] = v
			}
		}
	case "plans":
		for id, v := range scoped.Plans {
			if f.matchesID(v.ID) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Name, v.Interval) && (f.planID == "" || f.planID == v.ID) {
				out.Plans[id] = v
			}
		}
	case "tax_rules":
		for id, v := range scoped.Taxes {
			if f.matchesID(v.ID) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Name, v.Region) {
				out.Taxes[id] = v
			}
		}
	case "discounts":
		for id, v := range scoped.Discounts {
			if f.matchesID(v.ID) && f.matchesSearch(v.ID, v.Name, v.Kind) && (f.discountID == "" || f.discountID == v.ID) {
				out.Discounts[id] = v
			}
		}
	case "quotation_templates":
		for id, v := range scoped.QuoteTemplates {
			if f.matchesID(v.ID) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Name) && matchesAnyProduct(v.Lines, f.productID) {
				out.QuoteTemplates[id] = v
			}
		}
	case "quotations":
		for id, v := range scoped.Quotations {
			if f.matchesID(v.ID) && f.matchesStatus(v.Status) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Number, v.Title, v.Status) &&
				(f.customerID == "" || v.CustomerID == f.customerID) && (f.planID == "" || v.PlanID == f.planID) && matchesAnyProduct(v.Lines, f.productID) {
				out.Quotations[id] = v
			}
		}
	case "subscriptions":
		for id, v := range scoped.Subscriptions {
			if f.matchesID(v.ID) && f.matchesStatus(v.Status) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Number, v.Status) &&
				(f.customerID == "" || v.CustomerID == f.customerID) && (f.subscriptionID == "" || v.ID == f.subscriptionID) &&
				(f.planID == "" || v.PlanID == f.planID) && (f.discountID == "" || v.DiscountID == f.discountID) && matchesAnyProduct(v.Lines, f.productID) {
				out.Subscriptions[id] = v
			}
		}
	case "invoices":
		for id, v := range scoped.Invoices {
			if invoiceMatchesExport(f, v, scoped) {
				out.Invoices[id] = v
			}
		}
	case "payments":
		for id, v := range scoped.Payments {
			if paymentMatchesExport(f, v, scoped) {
				out.Payments[id] = v
			}
		}
	case "journal":
		for _, v := range scoped.Journal {
			if f.matchesID(v.ID) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.OperationKey, v.SourceType, v.SourceID, v.Account) &&
				(f.invoiceID == "" || v.SourceID == f.invoiceID) && (f.subscriptionID == "" || v.SourceID == f.subscriptionID) {
				out.Journal = append(out.Journal, v)
			}
		}
	case "audit":
		for _, v := range scoped.Audit {
			if f.matchesID(v.ID) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Action, v.Resource, v.ResourceID) {
				out.Audit = append(out.Audit, v)
			}
		}
	case "approvals":
		for id, v := range scoped.Approvals {
			if f.matchesID(v.ID) && f.matchesStatus(v.Status) && f.matchesDate(v.CreatedAt) && f.matchesSearch(v.ID, v.Action, v.ResourceType, v.ResourceID, v.Status) {
				out.Approvals[id] = v
			}
		}
	default:
		return nil, fmt.Errorf("%w: unsupported export resource %q", ErrValidation, f.resource)
	}
	closeExportRelationships(scoped, out, f.resource, exportRootIDs(out, f.resource))
	sort.Slice(out.Journal, func(i, j int) bool { return out.Journal[i].ID < out.Journal[j].ID })
	sort.Slice(out.Audit, func(i, j int) bool { return out.Audit[i].ID < out.Audit[j].ID })
	return out, nil
}

func businessExportState(state *State, businessID string) *State {
	out := NewState()
	if state == nil || businessID == "" {
		return out
	}
	if business, ok := state.Businesses[businessID]; ok {
		out.Businesses[businessID] = business
	}
	out.Customers = businessMap(state.Customers, businessID)
	out.Products = businessMap(state.Products, businessID)
	out.Plans = businessMap(state.Plans, businessID)
	out.Taxes = businessMap(state.Taxes, businessID)
	out.Discounts = businessMap(state.Discounts, businessID)
	out.QuoteTemplates = businessMap(state.QuoteTemplates, businessID)
	out.Quotations = businessMap(state.Quotations, businessID)
	out.Subscriptions = businessMap(state.Subscriptions, businessID)
	out.Invoices = businessMap(state.Invoices, businessID)
	out.Payments = businessMap(state.Payments, businessID)
	out.Allocations = businessMap(state.Allocations, businessID)
	out.Approvals = businessMap(state.Approvals, businessID)
	for _, v := range state.Journal {
		if v.BusinessID == businessID {
			out.Journal = append(out.Journal, v)
		}
	}
	for _, v := range state.Audit {
		if v.BusinessID == businessID {
			out.Audit = append(out.Audit, v)
		}
	}
	sort.Slice(out.Journal, func(i, j int) bool { return out.Journal[i].ID < out.Journal[j].ID })
	sort.Slice(out.Audit, func(i, j int) bool { return out.Audit[i].ID < out.Audit[j].ID })
	return out
}

func businessMap[T any](values map[string]T, businessID string) map[string]T {
	result := map[string]T{}
	for id, value := range values {
		var owner string
		switch v := any(value).(type) {
		case Customer:
			owner = v.BusinessID
		case Product:
			owner = v.BusinessID
		case Plan:
			owner = v.BusinessID
		case TaxRule:
			owner = v.BusinessID
		case Discount:
			owner = v.BusinessID
		case QuoteTemplate:
			owner = v.BusinessID
		case Quotation:
			owner = v.BusinessID
		case Subscription:
			owner = v.BusinessID
		case Invoice:
			owner = v.BusinessID
		case Payment:
			owner = v.BusinessID
		case PaymentAllocation:
			owner = v.BusinessID
		case Approval:
			owner = v.BusinessID
		}
		if owner == businessID {
			result[id] = value
		}
	}
	return result
}

func invoiceMatchesExport(f exportFilter, v Invoice, s *State) bool {
	customer := s.Customers[v.CustomerID]
	return f.matchesID(v.ID) && f.matchesStatus(v.Status) && f.matchesDate(v.CreatedAt) &&
		f.matchesSearch(v.ID, v.Number, v.Status, v.CustomerID, customer.Name, customer.Company, v.CalculationHash) &&
		(f.customerID == "" || v.CustomerID == f.customerID) && matchesInvoiceID(v, f.invoiceID) &&
		matchesInvoiceSubscription(v, f.subscriptionID) && matchesInvoiceProduct(v, f.productID) &&
		matchesInvoicePlan(v, f.planID) && matchesInvoiceDiscount(v, f.discountID)
}

func paymentMatchesExport(f exportFilter, v Payment, s *State) bool {
	if !f.matchesID(v.ID) || !f.matchesDate(v.ReceivedAt) || !f.matchesSearch(v.ID, v.Method, v.Provider, v.OperationKey, v.InvoiceID, v.CustomerID) ||
		(f.customerID != "" && v.CustomerID != f.customerID) || (f.invoiceID != "" && v.InvoiceID != f.invoiceID) {
		return false
	}
	if f.subscriptionID == "" && f.productID == "" && f.planID == "" && f.discountID == "" {
		return true
	}
	invoice, ok := s.Invoices[v.InvoiceID]
	if !ok {
		return false
	}
	return matchesInvoiceSubscription(invoice, f.subscriptionID) && matchesInvoiceProduct(invoice, f.productID) &&
		matchesInvoicePlan(invoice, f.planID) && matchesInvoiceDiscount(invoice, f.discountID)
}

func exportRootIDs(out *State, resource string) map[string]struct{} {
	ids := map[string]struct{}{}
	add := func(id string) {
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	switch resource {
	case "customers":
		for _, v := range out.Customers {
			add(v.ID)
		}
	case "products":
		for _, v := range out.Products {
			add(v.ID)
		}
	case "plans":
		for _, v := range out.Plans {
			add(v.ID)
		}
	case "tax_rules":
		for _, v := range out.Taxes {
			add(v.ID)
		}
	case "discounts":
		for _, v := range out.Discounts {
			add(v.ID)
		}
	case "quotation_templates":
		for _, v := range out.QuoteTemplates {
			add(v.ID)
		}
	case "quotations":
		for _, v := range out.Quotations {
			add(v.ID)
		}
	case "subscriptions":
		for _, v := range out.Subscriptions {
			add(v.ID)
		}
	case "invoices":
		for _, v := range out.Invoices {
			add(v.ID)
		}
	case "payments":
		for _, v := range out.Payments {
			add(v.ID)
		}
	case "approvals":
		for _, v := range out.Approvals {
			add(v.ID)
		}
	case "journal":
		for _, v := range out.Journal {
			add(v.ID)
		}
	case "audit":
		for _, v := range out.Audit {
			add(v.ID)
		}
	}
	return ids
}

func hasExportRootID(ids map[string]struct{}, id string) bool {
	_, ok := ids[id]
	return id != "" && ok
}

func closeExportRelationships(source, out *State, rootResource string, rootIDs map[string]struct{}) {
	for {
		before := exportRecordCount(out)
		for _, v := range out.Journal {
			includeReferencedRecord(source, out, v.SourceType, v.SourceID)
		}
		for _, v := range out.Audit {
			includeReferencedRecord(source, out, v.Resource, v.ResourceID)
		}
		for _, v := range out.Approvals {
			includeReferencedRecord(source, out, v.ResourceType, v.ResourceID)
		}
		for id, v := range source.Customers {
			if containsCustomer(out, v.ID) || customerReferenced(v.ID, out) {
				out.Customers[id] = v
			}
		}
		for id, v := range source.Products {
			if containsProduct(out, v.ID) || productReferenced(v.ID, out) {
				out.Products[id] = v
			}
		}
		for id, v := range source.Plans {
			if containsPlan(out, v.ID) || planReferenced(v.ID, out) {
				out.Plans[id] = v
			}
		}
		for id, v := range source.Taxes {
			if containsTax(out, v.ID) || taxReferenced(v.ID, out) {
				out.Taxes[id] = v
			}
		}
		for id, v := range source.Discounts {
			if containsDiscount(out, v.ID) || discountReferenced(v.ID, out) {
				out.Discounts[id] = v
			}
		}
		for id, v := range source.QuoteTemplates {
			if _, selected := out.QuoteTemplates[id]; selected || (rootResource == "products" && linesContainRootProduct(v.Lines, rootIDs)) {
				out.QuoteTemplates[id] = v
			}
		}
		for id, v := range source.Quotations {
			if _, selected := out.Quotations[id]; selected ||
				(rootResource == "customers" && hasExportRootID(rootIDs, v.CustomerID)) ||
				(rootResource == "products" && linesContainRootProduct(v.Lines, rootIDs)) ||
				(rootResource == "plans" && hasExportRootID(rootIDs, v.PlanID)) {
				out.Quotations[id] = v
			}
		}
		for id, v := range source.Subscriptions {
			if _, selected := out.Subscriptions[id]; selected ||
				invoiceReferencesSubscription(out, v.ID) ||
				(rootResource == "customers" && hasExportRootID(rootIDs, v.CustomerID)) ||
				(rootResource == "products" && linesContainRootProduct(v.Lines, rootIDs)) ||
				(rootResource == "plans" && hasExportRootID(rootIDs, v.PlanID)) ||
				(rootResource == "discounts" && hasExportRootID(rootIDs, v.DiscountID)) {
				out.Subscriptions[id] = v
			}
		}
		for id, v := range source.Invoices {
			if _, selected := out.Invoices[id]; selected ||
				(rootResource == "customers" && hasExportRootID(rootIDs, v.CustomerID)) ||
				(rootResource == "subscriptions" && hasExportRootID(rootIDs, v.SubscriptionID)) ||
				(rootResource == "plans" && hasExportRootID(rootIDs, v.Snapshot.PlanID)) ||
				(rootResource == "discounts" && hasExportRootID(rootIDs, v.Snapshot.DiscountID)) ||
				(rootResource == "products" && invoiceHasRootProduct(v, rootIDs)) ||
				(rootResource == "tax_rules" && invoiceHasRootTax(v, rootIDs)) ||
				(rootResource == "payments" && paymentReferencesInvoice(source, rootIDs, v.ID)) ||
				allocationReferencesInvoice(source, out, v.ID) {
				out.Invoices[id] = v
			}
		}
		for id, v := range source.Payments {
			if _, selected := out.Payments[id]; selected ||
				(rootResource == "customers" && hasExportRootID(rootIDs, v.CustomerID)) ||
				containsInvoice(out, v.InvoiceID) || allocationReferencesPayment(source, out, v.ID) {
				out.Payments[id] = v
			}
		}
		for id, v := range source.Allocations {
			if containsInvoice(out, v.InvoiceID) || containsPayment(out, v.PaymentID) {
				out.Allocations[id] = v
			}
		}
		for _, v := range source.Journal {
			if idReferencedByExport(out, v.SourceID) && !journalHasID(out.Journal, v.ID) {
				out.Journal = append(out.Journal, v)
			}
		}
		for _, v := range source.Audit {
			if idReferencedByExport(out, v.ResourceID) && !auditHasID(out.Audit, v.ID) {
				out.Audit = append(out.Audit, v)
			}
		}
		for id, v := range source.Approvals {
			if _, selected := out.Approvals[id]; selected || idReferencedByExport(out, v.ResourceID) {
				out.Approvals[id] = v
			}
		}
		if exportRecordCount(out) == before {
			return
		}
	}
}

func linesContainRootProduct(lines []SubscriptionLine, rootIDs map[string]struct{}) bool {
	for _, line := range lines {
		if hasExportRootID(rootIDs, line.ProductID) {
			return true
		}
	}
	return false
}

func invoiceHasRootProduct(invoice Invoice, rootIDs map[string]struct{}) bool {
	for _, line := range invoice.Snapshot.Lines {
		if hasExportRootID(rootIDs, line.ProductID) {
			return true
		}
	}
	return false
}

func invoiceHasRootTax(invoice Invoice, rootIDs map[string]struct{}) bool {
	for _, line := range invoice.Snapshot.Lines {
		if hasExportRootID(rootIDs, line.TaxID) {
			return true
		}
	}
	return false
}

func paymentReferencesInvoice(source *State, rootPaymentIDs map[string]struct{}, invoiceID string) bool {
	for _, payment := range source.Payments {
		if hasExportRootID(rootPaymentIDs, payment.ID) && payment.InvoiceID == invoiceID {
			return true
		}
	}
	for _, allocation := range source.Allocations {
		if hasExportRootID(rootPaymentIDs, allocation.PaymentID) && allocation.InvoiceID == invoiceID {
			return true
		}
	}
	return false
}

func invoiceReferencesSubscription(s *State, subscriptionID string) bool {
	for _, invoice := range s.Invoices {
		if invoice.SubscriptionID == subscriptionID {
			return true
		}
	}
	return false
}

func includeReferencedRecord(source, out *State, resource, id string) {
	if id == "" {
		return
	}
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "customer", "customers":
		if v, ok := source.Customers[id]; ok {
			out.Customers[id] = v
		}
	case "product", "products":
		if v, ok := source.Products[id]; ok {
			out.Products[id] = v
		}
	case "plan", "plans":
		if v, ok := source.Plans[id]; ok {
			out.Plans[id] = v
		}
	case "tax", "tax_rule", "tax_rules", "taxes":
		if v, ok := source.Taxes[id]; ok {
			out.Taxes[id] = v
		}
	case "discount", "discounts":
		if v, ok := source.Discounts[id]; ok {
			out.Discounts[id] = v
		}
	case "quote_template", "quotation_template", "quotation_templates":
		if v, ok := source.QuoteTemplates[id]; ok {
			out.QuoteTemplates[id] = v
		}
	case "quotation", "quotations":
		if v, ok := source.Quotations[id]; ok {
			out.Quotations[id] = v
		}
	case "subscription", "subscriptions":
		if v, ok := source.Subscriptions[id]; ok {
			out.Subscriptions[id] = v
		}
	case "invoice", "invoices":
		if v, ok := source.Invoices[id]; ok {
			out.Invoices[id] = v
		}
	case "payment", "payments":
		if v, ok := source.Payments[id]; ok {
			out.Payments[id] = v
		}
	}
}

func exportRecordCount(s *State) int {
	return len(s.Customers) + len(s.Products) + len(s.Plans) + len(s.Taxes) + len(s.Discounts) + len(s.QuoteTemplates) + len(s.Quotations) + len(s.Subscriptions) + len(s.Invoices) + len(s.Payments) + len(s.Allocations) + len(s.Journal) + len(s.Audit) + len(s.Approvals)
}

func containsCustomer(s *State, id string) bool { _, ok := s.Customers[id]; return id != "" && ok }
func containsProduct(s *State, id string) bool  { _, ok := s.Products[id]; return id != "" && ok }
func containsPlan(s *State, id string) bool     { _, ok := s.Plans[id]; return id != "" && ok }
func containsTax(s *State, id string) bool      { _, ok := s.Taxes[id]; return id != "" && ok }
func containsDiscount(s *State, id string) bool { _, ok := s.Discounts[id]; return id != "" && ok }
func containsSubscription(s *State, id string) bool {
	_, ok := s.Subscriptions[id]
	return id != "" && ok
}
func containsInvoice(s *State, id string) bool { _, ok := s.Invoices[id]; return id != "" && ok }
func containsPayment(s *State, id string) bool { _, ok := s.Payments[id]; return id != "" && ok }

func customerReferenced(id string, s *State) bool {
	for _, v := range s.Quotations {
		if v.CustomerID == id {
			return true
		}
	}
	for _, v := range s.Subscriptions {
		if v.CustomerID == id {
			return true
		}
	}
	for _, v := range s.Invoices {
		if v.CustomerID == id {
			return true
		}
	}
	for _, v := range s.Payments {
		if v.CustomerID == id {
			return true
		}
	}
	return false
}
func productReferenced(id string, s *State) bool {
	for _, v := range s.QuoteTemplates {
		if hasLineProduct(v.Lines, id) {
			return true
		}
	}
	for _, v := range s.Quotations {
		if hasLineProduct(v.Lines, id) {
			return true
		}
	}
	for _, v := range s.Subscriptions {
		if hasLineProduct(v.Lines, id) {
			return true
		}
	}
	for _, v := range s.Invoices {
		if matchesInvoiceProduct(v, id) {
			return true
		}
	}
	return false
}
func planReferenced(id string, s *State) bool {
	for _, v := range s.Quotations {
		if v.PlanID == id {
			return true
		}
	}
	for _, v := range s.Subscriptions {
		if v.PlanID == id {
			return true
		}
	}
	for _, v := range s.Invoices {
		if v.Snapshot.PlanID == id {
			return true
		}
	}
	return false
}
func taxReferenced(id string, s *State) bool {
	for _, v := range s.Invoices {
		if matchesInvoiceTax(v, id) {
			return true
		}
	}
	return false
}
func discountReferenced(id string, s *State) bool {
	for _, v := range s.Subscriptions {
		if v.DiscountID == id {
			return true
		}
	}
	for _, v := range s.Invoices {
		if v.Snapshot.DiscountID == id {
			return true
		}
	}
	return false
}
func hasLineProduct(lines []SubscriptionLine, id string) bool {
	for _, line := range lines {
		if line.ProductID == id {
			return true
		}
	}
	return false
}
func linesReferenceProduct(lines []SubscriptionLine, products map[string]Product) bool {
	for _, line := range lines {
		if _, ok := products[line.ProductID]; ok {
			return true
		}
	}
	return false
}
func invoiceHasSelectedProduct(v Invoice, products map[string]Product) bool {
	for _, line := range v.Snapshot.Lines {
		if _, ok := products[line.ProductID]; ok {
			return true
		}
	}
	return false
}
func invoiceHasSelectedTax(v Invoice, taxes map[string]TaxRule) bool {
	for _, line := range v.Snapshot.Lines {
		if _, ok := taxes[line.TaxID]; ok {
			return true
		}
	}
	return false
}
func allocationReferencesInvoice(source, out *State, invoiceID string) bool {
	for _, v := range source.Allocations {
		if v.InvoiceID == invoiceID && containsPayment(out, v.PaymentID) {
			return true
		}
	}
	return false
}
func allocationReferencesPayment(source, out *State, paymentID string) bool {
	for _, v := range source.Allocations {
		if v.PaymentID == paymentID && containsInvoice(out, v.InvoiceID) {
			return true
		}
	}
	return false
}
func idReferencedByExport(s *State, id string) bool {
	if id == "" {
		return false
	}
	_, c := s.Customers[id]
	_, p := s.Products[id]
	_, pl := s.Plans[id]
	_, t := s.Taxes[id]
	_, d := s.Discounts[id]
	_, qt := s.QuoteTemplates[id]
	_, q := s.Quotations[id]
	_, sub := s.Subscriptions[id]
	_, inv := s.Invoices[id]
	_, pay := s.Payments[id]
	return c || p || pl || t || d || qt || q || sub || inv || pay
}
func journalHasID(rows []JournalEntry, id string) bool {
	for _, v := range rows {
		if v.ID == id {
			return true
		}
	}
	return false
}
func auditHasID(rows []AuditEvent, id string) bool {
	for _, v := range rows {
		if v.ID == id {
			return true
		}
	}
	return false
}
