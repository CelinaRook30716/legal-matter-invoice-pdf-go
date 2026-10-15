package main

import (
	"strings"
	"testing"
	"time"
)

func TestInvoiceDecision(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	base := MatterOrder{MatterID: "M-42", ClientEmail: "client@example.org", Description: "Filing & review", AmountCents: 12550, SignedDocumentDelivered: true, Deadline: "2026-09-23"}
	for _, tc := range []struct {
		name      string
		delivered bool
		deadline  string
		due       bool
		reject    bool
	}{
		{"delivered and approaching", true, "2026-09-23", true, false},
		{"delivered and later", true, "2026-10-02", false, false},
		{"delivered and elapsed", true, "2026-09-19", false, false},
		{"unsigned delivery", false, "2026-09-23", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := base
			o.SignedDocumentDelivered = tc.delivered
			o.Deadline = tc.deadline
			got, err := decide(o, now)
			if (err != nil) != tc.reject {
				t.Fatalf("error = %v, reject = %v", err, tc.reject)
			}
			if !tc.reject && (got.FollowUp != tc.due || !strings.Contains(got.InvoiceHTML, "Filing &amp; review")) {
				t.Fatalf("decision = %+v", got)
			}
		})
	}
}
