package appdata

import "testing"

func boolPtr(b bool) *bool { return &b }

const urlPath = "/users_orders"

// Test that mappings are loaded into a fresh RuntimeIndex and basic
// method+URL+query+body matching works.
func TestRuntimeIndexAddAndFindBasic(t *testing.T) {
	ri := NewRuntimeIndex()

	m1 := Mapping{
		ID: "m1",
		Request: Request{
			Method:     "POST",
			URLPattern: urlPath,
			// match userId=123
			QueryParams: map[string]string{
				"userId": "123",
			},
			Body: map[string]any{
				"orderType": "ALL",
			},
		},
		Response: Response{Status: 201},
		Metadata: Metadata{Enabled: boolPtr(true)},
	}

	m2 := Mapping{
		ID: "m2",
		Request: Request{
			Method:     "POST",
			URLPattern: "/users_orders",
			// match userId=456
			QueryParams: map[string]string{
				"userId": "456",
			},
			Body: map[string]any{
				"orderType": "ALL",
			},
		},
		Response: Response{Status: 202},
		Metadata: Metadata{Enabled: boolPtr(true)},
	}

	if err := ri.Add(m1); err != nil {
		t.Fatalf("Add(m1) error = %v", err)
	}
	if err := ri.Add(m2); err != nil {
		t.Fatalf("Add(m2) error = %v", err)
	}

	// Request with userId=123 should pick m1.
	req1 := IncomingRequest{
		Method: "POST",
		URL:    "/users_orders",
		Query: map[string][]string{
			"userId": {"123"},
		},
		Body: map[string]any{
			"orderType": "ALL",
		},
	}

	got1, ok := ri.FindBestMatch(req1)
	if !ok {
		t.Fatalf("FindBestMatch(req1) = no match, want m1")
	}
	if got1.ID != "m1" {
		t.Fatalf("FindBestMatch(req1).ID = %q, want %q", got1.ID, "m1")
	}

	// Request with userId=456 should pick m2.
	req2 := IncomingRequest{
		Method: "POST",
		URL:    "/users_orders",
		Query: map[string][]string{
			"userId": {"456"},
		},
		Body: map[string]any{
			"orderType": "ALL",
		},
	}

	got2, ok := ri.FindBestMatch(req2)
	if !ok {
		t.Fatalf("FindBestMatch(req2) = no match, want m2")
	}
	if got2.ID != "m2" {
		t.Fatalf("FindBestMatch(req2).ID = %q, want %q", got2.ID, "m2")
	}
}

// Test that when two mappings both match, the one with higher specificity
// (e.g. has body constraints) is chosen when priority is equal.
func TestRuntimeIndexFindBestMatchSpecificityBeatsGeneric(t *testing.T) {
	ri := NewRuntimeIndex()

	// Generic: matches on method+URL only.
	generic := Mapping{
		ID: "generic",
		Request: Request{
			Method:     "POST",
			URLPattern: urlPath,
		},
		Response: Response{Status: 200},
		Metadata: Metadata{Enabled: boolPtr(true)},
	}

	// Specific: requires body.orderType == "ALL".
	specific := Mapping{
		ID: "specific",
		Request: Request{
			Method:     "POST",
			URLPattern: "/orders",
			Body: map[string]any{
				"orderType": "ALL",
			},
		},
		Response: Response{Status: 201},
		Metadata: Metadata{Enabled: boolPtr(true)},
	}

	if err := ri.Add(generic); err != nil {
		t.Fatalf("Add(generic) error = %v", err)
	}
	if err := ri.Add(specific); err != nil {
		t.Fatalf("Add(specific) error = %v", err)
	}

	req := IncomingRequest{
		Method: "POST",
		URL:    "/orders",
		Body: map[string]any{
			"orderType": "ALL",
		},
	}

	got, ok := ri.FindBestMatch(req)
	if !ok {
		t.Fatalf("FindBestMatch(req) = no match, want specific")
	}
	if got.ID != "specific" {
		t.Fatalf("FindBestMatch(req).ID = %q, want %q", got.ID, "specific")
	}
}

// Test that disabled mappings are not added / matched.
func TestRuntimeIndexDisabledMappingIgnored(t *testing.T) {
	ri := NewRuntimeIndex()

	disabled := false
	m := Mapping{
		ID:       "disabled",
		Priority: 100,
		Request: Request{
			Method:     "GET",
			URLPattern: urlPath,
		},
		Response: Response{Status: 200},
		Metadata: Metadata{Enabled: &disabled},
	}

	if err := ri.Add(m); err != nil {
		t.Fatalf("Add(disabled) error = %v", err)
	}

	if got := ri.Count(); got != 0 {
		t.Fatalf("Count() = %d, want 0 (disabled mapping should not be indexed)", got)
	}

	req := IncomingRequest{
		Method: "GET",
		URL:    "/health",
	}
	if _, ok := ri.FindBestMatch(req); ok {
		t.Fatalf("FindBestMatch on disabled mapping returned a match, want no match")
	}
}
