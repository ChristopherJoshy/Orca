package orca

import "time"

func subtractMoney(left, right string) string {
	l, err := ParseDecimal(left)
	if err != nil {
		return "0.00"
	}
	r, err := ParseDecimal(right)
	if err != nil {
		return "0.00"
	}
	return amountString(l.Sub(r))
}

func auditEvent(actor Actor, action, resource, resourceID string, details any) AuditEvent {
	return AuditEvent{ID: mustNewUUID(), BusinessID: actor.BusinessID, ActorID: actor.UserID, Action: action, Resource: resource, ResourceID: resourceID, Details: details, CreatedAt: time.Now().UTC()}
}
