package orca

import (
	"fmt"
)

// canonicalCommercialLines resolves catalog prices, taxes, names, and variants
// on the server before a quote or subscription stores its commercial terms.
// Browser-supplied totals are suggestions only and never establish price truth.
func canonicalCommercialLines(st *State, plan Plan, requested []SubscriptionLine, planQuantity int) ([]SubscriptionLine, error) {
	if plan.Archived {
		return nil, ErrNotFound
	}
	productLines := make([]SubscriptionLine, 0, len(requested))
	var planLine *SubscriptionLine
	for _, line := range requested {
		if line.Quantity < 1 {
			return nil, fmt.Errorf("%w: line quantity must be positive", ErrValidation)
		}
		if line.ProductID == "" {
			if planLine != nil {
				return nil, fmt.Errorf("%w: a commercial document can contain only one plan line", ErrValidation)
			}
			if line.Quantity < plan.MinimumQuantity {
				return nil, fmt.Errorf("%w: quantity is below the plan minimum", ErrValidation)
			}
			canonical := SubscriptionLine{Name: plan.Name, Quantity: line.Quantity, UnitPrice: money2(plan.SalesPrice), TaxRate: rate2(plan.TaxRate)}
			planLine = &canonical
			continue
		}
		product, ok := st.Products[line.ProductID]
		if !ok || product.BusinessID != plan.BusinessID || product.Archived {
			return nil, ErrNotFound
		}
		unitPrice := money2(product.SalesPrice)
		variantName := ""
		if line.Variant != "" {
			found := false
			for _, variant := range product.Variants {
				if line.Variant == variant.Name || line.Variant == variant.Value {
					base, err := ParseDecimal(unitPrice)
					if err != nil {
						return nil, err
					}
					adjustment, err := ParseDecimal(variant.Adjustment)
					if err != nil {
						return nil, err
					}
					unitPrice = amountString(base.Add(adjustment))
					variantName = variant.Value
					if variantName == "" {
						variantName = variant.Name
					}
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("%w: selected product variant does not exist", ErrValidation)
			}
		}
		productLines = append(productLines, SubscriptionLine{ProductID: product.ID, Name: product.Name, Quantity: line.Quantity, UnitPrice: unitPrice, TaxRate: rate2(product.TaxRate), Variant: variantName})
	}
	if planLine == nil {
		if planQuantity < plan.MinimumQuantity {
			return nil, fmt.Errorf("%w: quantity is below the plan minimum", ErrValidation)
		}
		canonical := SubscriptionLine{Name: plan.Name, Quantity: planQuantity, UnitPrice: money2(plan.SalesPrice), TaxRate: rate2(plan.TaxRate)}
		planLine = &canonical
	}
	// Put the plan first so downstream minimum-quantity checks and summaries
	// always inspect the recurring plan quantity, regardless of request order.
	lines := make([]SubscriptionLine, 0, len(productLines)+1)
	lines = append(lines, *planLine)
	lines = append(lines, productLines...)
	return lines, nil
}
