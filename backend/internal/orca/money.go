package orca

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var ErrInvalidMoney = errors.New("money must be a decimal string")

// ParseDecimal rejects JSON numeric values at the API boundary; callers must
// provide string values so no binary floating-point conversion is involved.
func ParseDecimal(s string) (decimal.Decimal, error) {
	if strings.TrimSpace(s) != s || s == "" {
		return decimal.Zero, ErrInvalidMoney
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, ErrInvalidMoney
	}
	return d, nil
}

func decimalString(d decimal.Decimal) string { return d.String() }
func amountString(d decimal.Decimal) string  { return d.Round(2).StringFixed(2) }

// Money is a decimal string on the wire, including zero-valued amounts.
type Money string

func (m Money) Decimal() (decimal.Decimal, error) { return ParseDecimal(string(m)) }
func (m Money) MarshalJSON() ([]byte, error)      { return json.Marshal(string(m)) }
func (m *Money) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return ErrInvalidMoney
	}
	if _, err := ParseDecimal(s); err != nil {
		return err
	}
	*m = Money(s)
	return nil
}

func canonicalHash(v any) (string, []byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}

type CalculationInputLine struct {
	Product     Product
	Quantity    int
	UnitPrice   string
	TaxRate     string
	Description string
	Variant     string
	PlanID      string
	PlanVersion int64
	TaxID       string
	TaxVersion  int64
}

func Calculate(input []CalculationInputLine, discount *Discount, businessID, subscriptionID string, start, end time.Time, timezone ...string) (CalculationSnapshot, string, error) {
	if len(input) == 0 {
		return CalculationSnapshot{}, "", errors.New("invoice requires at least one line")
	}
	base := make([]decimal.Decimal, len(input))
	taxRates := make([]decimal.Decimal, len(input))
	unitPrices := make([]decimal.Decimal, len(input))
	for i, line := range input {
		if line.Quantity <= 0 {
			return CalculationSnapshot{}, "", fmt.Errorf("line %d quantity must be positive", i+1)
		}
		price, err := ParseDecimal(line.UnitPrice)
		if err != nil || price.IsNegative() {
			return CalculationSnapshot{}, "", fmt.Errorf("line %d has invalid unit price", i+1)
		}
		rate, err := ParseDecimal(line.TaxRate)
		if err != nil || rate.IsNegative() || rate.GreaterThan(decimal.NewFromInt(100)) {
			return CalculationSnapshot{}, "", fmt.Errorf("line %d has invalid tax rate", i+1)
		}
		unitPrices[i], taxRates[i] = price, rate
		base[i] = price.Mul(decimal.NewFromInt(int64(line.Quantity)))
	}
	eligible := make([]bool, len(input))
	var eligibleTotal decimal.Decimal
	eligibleQuantity := 0
	for i, line := range input {
		eligible[i] = discount != nil && discountEligibleLine(*discount, line.Product.ID)
		if eligible[i] {
			eligibleTotal = eligibleTotal.Add(base[i].Round(2))
			eligibleQuantity += line.Quantity
		}
	}
	discountAmounts := make([]decimal.Decimal, len(input))
	if discount != nil && discountEligible(*discount, eligibleTotal, eligibleQuantity, start) && eligibleTotal.IsPositive() {
		value, err := ParseDecimal(discount.Value)
		if err != nil || value.IsNegative() {
			return CalculationSnapshot{}, "", errors.New("discount has invalid value")
		}
		switch discount.Kind {
		case "percentage":
			if value.GreaterThan(decimal.NewFromInt(100)) {
				return CalculationSnapshot{}, "", errors.New("percentage discount exceeds 100")
			}
			for i := range input {
				if eligible[i] {
					discountAmounts[i] = minDecimal(base[i].Mul(value).Div(decimal.NewFromInt(100)).Round(2), base[i].Round(2))
				}
			}
		case "fixed":
			target := minDecimal(value.Round(2), eligibleTotal.Round(2))
			totalRounded := eligibleTotal.Round(2)
			type remainder struct {
				index    int
				fraction decimal.Decimal
			}
			remainders := make([]remainder, 0, len(input))
			allocated := decimal.Zero
			for i := range input {
				if !eligible[i] {
					continue
				}
				lineBase := base[i].Round(2)
				if lineBase.IsZero() {
					continue
				}
				exact := target.Mul(lineBase).Div(totalRounded)
				share := minDecimal(exact.Truncate(2), lineBase)
				discountAmounts[i] = share
				allocated = allocated.Add(share)
				remainders = append(remainders, remainder{index: i, fraction: exact.Sub(share)})
			}
			sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].fraction.GreaterThan(remainders[j].fraction) })
			remaining := target.Sub(allocated)
			cent := decimal.RequireFromString("0.01")
			for remaining.IsPositive() {
				progress := false
				for _, r := range remainders {
					capacity := base[r.index].Round(2).Sub(discountAmounts[r.index])
					if capacity.GreaterThanOrEqual(cent) {
						discountAmounts[r.index] = discountAmounts[r.index].Add(cent)
						remaining = remaining.Sub(cent)
						progress = true
						if remaining.IsZero() {
							break
						}
					}
				}
				if !progress {
					return CalculationSnapshot{}, "", errors.New("fixed discount could not be allocated safely")
				}
			}
		default:
			return CalculationSnapshot{}, "", errors.New("discount kind must be fixed or percentage")
		}
	}

	lines := make([]CalculationLine, 0, len(input))
	var lineSubtotal, discountTotal, taxTotal, total decimal.Decimal
	for i, line := range input {
		raw := base[i]
		disc := discountAmounts[i].Round(2)
		taxable := maxZero(raw.Sub(disc)).Round(2)
		tax := taxable.Mul(taxRates[i]).Div(decimal.NewFromInt(100)).Round(2)
		lineTotal := taxable.Add(tax).Round(2)
		lineSubtotal = lineSubtotal.Add(raw.Round(2))
		discountTotal = discountTotal.Add(disc)
		taxTotal = taxTotal.Add(tax)
		total = total.Add(lineTotal)
		steps := []string{
			fmt.Sprintf("%d × %s = %s", line.Quantity, decimalString(unitPrices[i]), amountString(raw)),
			fmt.Sprintf("Discount %s; taxable amount %s", amountString(disc), amountString(taxable)),
			fmt.Sprintf("Tax %s%% on %s = %s", decimalString(taxRates[i]), amountString(taxable), amountString(tax)),
			fmt.Sprintf("Line total %s", amountString(lineTotal)),
		}
		lines = append(lines, CalculationLine{
			ProductID: line.Product.ID, ProductVersion: line.Product.Version, Description: line.Description, Variant: line.Variant,
			PlanID: line.PlanID, PlanVersion: line.PlanVersion, TaxID: line.TaxID, TaxVersion: line.TaxVersion,
			Quantity: line.Quantity, UnitPrice: decimalString(unitPrices[i]), RawSubtotal: amountString(raw),
			DiscountAmount: amountString(disc), TaxableAmount: amountString(taxable), TaxRate: decimalString(taxRates[i]),
			TaxAmount: amountString(tax), LineTotal: amountString(lineTotal), Explanation: steps,
		})
	}
	snapshot := CalculationSnapshot{
		EngineVersion: EngineVersion, Currency: CurrencyINR, BusinessID: businessID,
		SubscriptionID: subscriptionID, PeriodStart: start.UTC(), PeriodEnd: end.UTC(),
		Subtotal: amountString(lineSubtotal), DiscountTotal: amountString(discountTotal),
		TaxTotal: amountString(taxTotal), Total: amountString(total), Lines: lines,
		Explanation: []string{"Amounts use exact decimal arithmetic.", "Discount is applied before tax.", "Each line is rounded half-up to two INR decimal places.", "Document total is the sum of stored rounded line totals."},
	}
	if len(timezone) > 0 {
		snapshot.Timezone = timezone[0]
	}
	if len(input) > 0 {
		snapshot.PlanID = input[0].PlanID
		snapshot.PlanVersion = input[0].PlanVersion
	}
	if discount != nil && discountEligible(*discount, eligibleTotal, eligibleQuantity, start) {
		snapshot.DiscountID = discount.ID
		snapshot.DiscountVersion = discount.Version
	}
	hash, _, err := canonicalHash(snapshot)
	return snapshot, hash, err
}

func minDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

func maxZero(value decimal.Decimal) decimal.Decimal {
	if value.IsNegative() {
		return decimal.Zero
	}
	return value
}

func discountEligibleLine(d Discount, productID string) bool {
	if d.Scope == "subscription" || len(d.ProductIDs) == 0 {
		return true
	}
	for _, id := range d.ProductIDs {
		if id == productID {
			return true
		}
	}
	return false
}

func discountEligible(d Discount, purchase decimal.Decimal, quantity int, at time.Time) bool {
	minimum, err := ParseDecimal(d.MinimumPurchase)
	if err != nil {
		minimum = decimal.Zero
	}
	if purchase.LessThan(minimum) || quantity < d.MinimumQuantity {
		return false
	}
	if d.StartsAt != nil && at.Before(*d.StartsAt) {
		return false
	}
	if d.EndsAt != nil && at.After(*d.EndsAt) {
		return false
	}
	if d.MaxUses > 0 && d.UseCount >= d.MaxUses {
		return false
	}
	return true
}
