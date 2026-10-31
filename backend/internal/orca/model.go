package orca

import (
	"encoding/json"
	"time"
)

const (
	CurrencyINR   = "INR"
	EngineVersion = "orca-billing/1.0.0"
)

type User struct {
	ID              string    `json:"id"`
	Email           string    `json:"email"`
	EmailNormalized string    `json:"emailNormalized"`
	PasswordHash    string    `json:"-"`
	Verified        bool      `json:"verified"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Business struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Currency          string `json:"currency"`
	Timezone          string `json:"timezone"`
	NextInvoiceNumber int64  `json:"nextInvoiceNumber"`
}

type Membership struct {
	BusinessID string     `json:"businessId"`
	UserID     string     `json:"userId"`
	Role       string     `json:"role"`
	CustomerID string     `json:"customerId,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

type Customer struct {
	ID         string            `json:"id"`
	BusinessID string            `json:"businessId"`
	UserID     string            `json:"userId,omitempty"`
	Name       string            `json:"name"`
	Email      string            `json:"email"`
	Company    string            `json:"company"`
	Phone      string            `json:"phone"`
	Address    map[string]string `json:"address,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
}

type Variant struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	Adjustment string `json:"adjustment"`
}

type Product struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"businessId"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	SalesPrice string    `json:"salesPrice"`
	CostPrice  string    `json:"costPrice"`
	Notes      string    `json:"notes"`
	Variants   []Variant `json:"variants"`
	TaxRate    string    `json:"taxRate"`
	Version    int64     `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Plan struct {
	ID              string    `json:"id"`
	BusinessID      string    `json:"businessId"`
	Name            string    `json:"name"`
	Interval        string    `json:"interval"`
	SalesPrice      string    `json:"salesPrice"`
	MinimumQuantity int       `json:"minimumQuantity"`
	AvailableFrom   string    `json:"availableFrom,omitempty"`
	AvailableUntil  string    `json:"availableUntil,omitempty"`
	TaxRate         string    `json:"taxRate"`
	AutoClose       bool      `json:"autoClose"`
	AllowEarlyClose bool      `json:"allowEarlyClose"`
	AllowPause      bool      `json:"allowPause"`
	AllowRenew      bool      `json:"allowRenew"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Discount struct {
	ID              string     `json:"id"`
	BusinessID      string     `json:"businessId"`
	Name            string     `json:"name"`
	Kind            string     `json:"kind"`
	Value           string     `json:"value"`
	MinimumPurchase string     `json:"minimumPurchase"`
	MinimumQuantity int        `json:"minimumQuantity"`
	StartsAt        *time.Time `json:"startsAt,omitempty"`
	EndsAt          *time.Time `json:"endsAt,omitempty"`
	MaxUses         int64      `json:"maxUses,omitempty"`
	UseCount        int64      `json:"useCount"`
	Scope           string     `json:"scope"`
	ProductIDs      []string   `json:"productIds"`
	Version         int64      `json:"version"`
}

type QuoteTemplate struct {
	ID         string             `json:"id"`
	BusinessID string             `json:"businessId"`
	Name       string             `json:"name"`
	ValidDays  int                `json:"validDays"`
	Lines      []SubscriptionLine `json:"lines"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type Quotation struct {
	ID         string             `json:"id"`
	BusinessID string             `json:"businessId"`
	CustomerID string             `json:"customerId"`
	PlanID     string             `json:"planId,omitempty"`
	Number     string             `json:"number"`
	Title      string             `json:"title"`
	Status     string             `json:"status"`
	ValidUntil string             `json:"validUntil,omitempty"`
	Lines      []SubscriptionLine `json:"lines"`
	Amount     string             `json:"amount"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type TaxRule struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"businessId"`
	Name       string    `json:"name"`
	Region     string    `json:"region"`
	Rate       string    `json:"rate"`
	Active     bool      `json:"active"`
	Version    int64     `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
}

type SubscriptionLine struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice string `json:"unitPrice"`
	TaxRate   string `json:"taxRate"`
	Variant   string `json:"variant,omitempty"`
}

type Subscription struct {
	ID               string             `json:"id"`
	BusinessID       string             `json:"businessId"`
	CustomerID       string             `json:"customerId"`
	PlanID           string             `json:"planId"`
	DiscountID       string             `json:"discountId,omitempty"`
	Number           string             `json:"number"`
	Status           string             `json:"status"`
	StartDate        string             `json:"startDate"`
	ExpiryDate       string             `json:"expiryDate,omitempty"`
	BillingCursor    *time.Time         `json:"billingCursor,omitempty"`
	PaymentTermsDays int                `json:"paymentTermsDays"`
	Lines            []SubscriptionLine `json:"lines"`
	PauseIntervals   []PauseInterval    `json:"pauseIntervals,omitempty"`
	Version          int64              `json:"version"`
	CreatedAt        time.Time          `json:"createdAt"`
}

type PauseInterval struct {
	Start time.Time  `json:"start"`
	End   *time.Time `json:"end,omitempty"`
}

type CalculationLine struct {
	ProductID      string   `json:"productId"`
	ProductVersion int64    `json:"productVersion"`
	PlanID         string   `json:"planId,omitempty"`
	PlanVersion    int64    `json:"planVersion,omitempty"`
	TaxID          string   `json:"taxId,omitempty"`
	TaxVersion     int64    `json:"taxVersion,omitempty"`
	Description    string   `json:"description"`
	Variant        string   `json:"variant,omitempty"`
	Quantity       int      `json:"quantity"`
	UnitPrice      string   `json:"unitPrice"`
	RawSubtotal    string   `json:"rawSubtotal"`
	DiscountAmount string   `json:"discountAmount"`
	TaxableAmount  string   `json:"taxableAmount"`
	TaxRate        string   `json:"taxRate"`
	TaxAmount      string   `json:"taxAmount"`
	LineTotal      string   `json:"lineTotal"`
	Explanation    []string `json:"explanation"`
}

type CalculationSnapshot struct {
	EngineVersion   string            `json:"engineVersion"`
	Currency        string            `json:"currency"`
	BusinessID      string            `json:"businessId"`
	SubscriptionID  string            `json:"subscriptionId,omitempty"`
	PlanID          string            `json:"planId,omitempty"`
	PlanVersion     int64             `json:"planVersion,omitempty"`
	PeriodStart     time.Time         `json:"periodStart"`
	PeriodEnd       time.Time         `json:"periodEnd"`
	Timezone        string            `json:"timezone"`
	DiscountID      string            `json:"discountId,omitempty"`
	DiscountVersion int64             `json:"discountVersion,omitempty"`
	Subtotal        string            `json:"subtotal"`
	DiscountTotal   string            `json:"discountTotal"`
	TaxTotal        string            `json:"taxTotal"`
	Total           string            `json:"total"`
	Lines           []CalculationLine `json:"lines"`
	Explanation     []string          `json:"explanation"`
}

type Invoice struct {
	ID              string              `json:"id"`
	BusinessID      string              `json:"businessId"`
	SubscriptionID  string              `json:"subscriptionId,omitempty"`
	CustomerID      string              `json:"customerId"`
	Number          string              `json:"number"`
	PeriodStart     time.Time           `json:"periodStart"`
	PeriodEnd       time.Time           `json:"periodEnd"`
	Status          string              `json:"status"`
	Currency        string              `json:"currency"`
	Subtotal        string              `json:"subtotal"`
	DiscountTotal   string              `json:"discountTotal"`
	TaxTotal        string              `json:"taxTotal"`
	Total           string              `json:"total"`
	Outstanding     string              `json:"outstanding"`
	DueAt           time.Time           `json:"dueAt"`
	Overdue         bool                `json:"overdue"`
	Snapshot        CalculationSnapshot `json:"snapshot"`
	CalculationHash string              `json:"calculationHash"`
	Version         int64               `json:"version"`
	CreatedAt       time.Time           `json:"createdAt"`
}

type Payment struct {
	ID           string    `json:"id"`
	BusinessID   string    `json:"businessId"`
	CustomerID   string    `json:"customerId"`
	Method       string    `json:"method"`
	Amount       string    `json:"amount"`
	ReceivedAt   time.Time `json:"receivedAt"`
	Note         string    `json:"note"`
	Provider     string    `json:"provider"`
	OperationKey string    `json:"operationKey"`
	InvoiceID    string    `json:"invoiceId"`
	CreatedAt    time.Time `json:"createdAt"`
}

type PaymentAllocation struct {
	BusinessID string    `json:"businessId"`
	PaymentID  string    `json:"paymentId"`
	InvoiceID  string    `json:"invoiceId"`
	Amount     string    `json:"amount"`
	CreatedAt  time.Time `json:"createdAt"`
}

// StripeCheckout binds a provider session to one ORCA invoice and immutable
// amount. It is persisted before the network call so retries reuse the same
// idempotency key and webhook reconciliation has a trusted local reference.
type StripeCheckout struct {
	ID                string     `json:"id"`
	BusinessID        string     `json:"businessId"`
	InvoiceID         string     `json:"invoiceId"`
	CustomerID        string     `json:"customerId"`
	Amount            string     `json:"amount"`
	Currency          string     `json:"currency"`
	IdempotencyKey    string     `json:"idempotencyKey"`
	ProviderSessionID string     `json:"providerSessionId,omitempty"`
	URL               string     `json:"url,omitempty"`
	Status            string     `json:"status"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// StripeReconciliation is a durable system-level outbox item for successful
// provider sessions that cannot safely be applied to an ORCA receivable.
type StripeReconciliation struct {
	ID              string          `json:"id"`
	ProviderEventID string          `json:"providerEventId"`
	EventType       string          `json:"eventType"`
	SessionID       string          `json:"sessionId"`
	CheckoutID      string          `json:"checkoutId,omitempty"`
	BusinessID      string          `json:"businessId,omitempty"`
	Reason          string          `json:"reason"`
	Payload         json.RawMessage `json:"payload"`
	CreatedAt       time.Time       `json:"createdAt"`
}

type JournalEntry struct {
	ID           string    `json:"id"`
	BusinessID   string    `json:"businessId"`
	OperationKey string    `json:"operationKey"`
	SourceType   string    `json:"sourceType"`
	SourceID     string    `json:"sourceId"`
	Account      string    `json:"account"`
	Debit        string    `json:"debit"`
	Credit       string    `json:"credit"`
	Currency     string    `json:"currency"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Approval struct {
	ID              string         `json:"id"`
	BusinessID      string         `json:"businessId"`
	ActorUserID     string         `json:"actorUserId"`
	Action          string         `json:"action"`
	ResourceType    string         `json:"resourceType"`
	ResourceID      string         `json:"resourceId,omitempty"`
	Inputs          any            `json:"inputs"`
	InputsHash      string         `json:"inputsHash"`
	ResourceVersion int64          `json:"resourceVersion"`
	Status          string         `json:"status"`
	ExpiresAt       time.Time      `json:"expiresAt"`
	DecidedBy       string         `json:"decidedBy,omitempty"`
	DecidedAt       *time.Time     `json:"decidedAt,omitempty"`
	ExecutionResult map[string]any `json:"executionResult,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
}

type Session struct {
	UserID     string    `json:"userId"`
	BusinessID string    `json:"businessId"`
	CustomerID string    `json:"customerId,omitempty"`
	Role       string    `json:"role"`
	CSRFToken  string    `json:"csrfToken"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type AuditEvent struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"businessId"`
	ActorID    string    `json:"actorId,omitempty"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resourceId,omitempty"`
	Details    any       `json:"details,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type OutboxEvent struct {
	ID             string         `json:"id"`
	BusinessID     string         `json:"businessId"`
	Type           string         `json:"type"`
	IdempotencyKey string         `json:"idempotencyKey"`
	Payload        map[string]any `json:"payload"`
	CreatedAt      time.Time      `json:"createdAt"`
	DeliveredAt    *time.Time     `json:"deliveredAt,omitempty"`
	Attempts       int            `json:"attempts"`
	LastError      string         `json:"lastError,omitempty"`
}

type AgentRun struct {
	ID             string    `json:"id"`
	BusinessID     string    `json:"businessId"`
	UserID         string    `json:"userId"`
	ConversationID string    `json:"conversationId"`
	Status         string    `json:"status"`
	Message        string    `json:"message"`
	Transcript     []any     `json:"transcript"`
	Outcome        any       `json:"outcome,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type OAuthGrant struct {
	ID         string     `json:"id"`
	BusinessID string     `json:"businessId"`
	UserID     string     `json:"userId"`
	ClientID   string     `json:"clientId"`
	Scopes     []string   `json:"scopes"`
	TokenHash  string     `json:"-"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

type ExportJob struct {
	ID          string         `json:"id"`
	BusinessID  string         `json:"businessId"`
	RequestedBy string         `json:"requestedBy"`
	Status      string         `json:"status"`
	Filters     map[string]any `json:"filters"`
	SnapshotAt  *time.Time     `json:"snapshotAt,omitempty"`
	Manifest    map[string]any `json:"manifest"`
	ArchivePath string         `json:"archivePath,omitempty"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type Actor struct {
	UserID     string `json:"userId"`
	BusinessID string `json:"businessId"`
	CustomerID string `json:"customerId,omitempty"`
	Role       string `json:"role"`
}

type State struct {
	Users                 map[string]User                 `json:"users"`
	Businesses            map[string]Business             `json:"businesses"`
	Memberships           map[string]Membership           `json:"memberships"`
	Customers             map[string]Customer             `json:"customers"`
	Products              map[string]Product              `json:"products"`
	Plans                 map[string]Plan                 `json:"plans"`
	Discounts             map[string]Discount             `json:"discounts"`
	QuoteTemplates        map[string]QuoteTemplate        `json:"quoteTemplates"`
	Quotations            map[string]Quotation            `json:"quotations"`
	Taxes                 map[string]TaxRule              `json:"taxes"`
	Subscriptions         map[string]Subscription         `json:"subscriptions"`
	Invoices              map[string]Invoice              `json:"invoices"`
	Payments              map[string]Payment              `json:"payments"`
	Allocations           map[string]PaymentAllocation    `json:"allocations"`
	StripeCheckouts       map[string]StripeCheckout       `json:"stripeCheckouts"`
	StripeReconciliations map[string]StripeReconciliation `json:"stripeReconciliations"`
	Journal               []JournalEntry                  `json:"journal"`
	Approvals             map[string]Approval             `json:"approvals"`
	Sessions              map[string]Session              `json:"sessions"`
	Audit                 []AuditEvent                    `json:"audit"`
	Outbox                []OutboxEvent                   `json:"outbox"`
	AgentRuns             map[string]AgentRun             `json:"agentRuns"`
	OAuthGrants           map[string]OAuthGrant           `json:"oauthGrants"`
	Exports               map[string]ExportJob            `json:"exports"`
	WebhookIDs            map[string]bool                 `json:"webhookIDs"`
	EmailVerifications    map[string]AuthToken            `json:"emailVerifications"`
	ResetTokens           map[string]AuthToken            `json:"resetTokens"`
}

type AuthToken struct {
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func NewState() *State {
	return &State{
		Users: map[string]User{}, Businesses: map[string]Business{}, Memberships: map[string]Membership{},
		Customers: map[string]Customer{}, Products: map[string]Product{}, Plans: map[string]Plan{},
		Discounts: map[string]Discount{}, QuoteTemplates: map[string]QuoteTemplate{}, Subscriptions: map[string]Subscription{},
		Quotations: map[string]Quotation{}, Taxes: map[string]TaxRule{},
		Invoices: map[string]Invoice{}, Payments: map[string]Payment{}, Allocations: map[string]PaymentAllocation{}, StripeCheckouts: map[string]StripeCheckout{}, StripeReconciliations: map[string]StripeReconciliation{}, Approvals: map[string]Approval{}, Sessions: map[string]Session{},
		AgentRuns: map[string]AgentRun{}, OAuthGrants: map[string]OAuthGrant{}, Exports: map[string]ExportJob{},
		WebhookIDs: map[string]bool{}, EmailVerifications: map[string]AuthToken{}, ResetTokens: map[string]AuthToken{},
	}
}

func membershipKey(businessID, userID string) string { return businessID + ":" + userID }
