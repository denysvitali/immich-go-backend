package server

import "testing"

func TestPartnerIDFromPath(t *testing.T) {
	tests := []struct {
		path   string
		wantID string
		wantOK bool
	}{
		{path: "/api/partners/00000000-0000-0000-0000-000000000001", wantID: "00000000-0000-0000-0000-000000000001", wantOK: true},
		{path: "/api/partners", wantOK: false},
		{path: "/api/partners/", wantOK: false},
		{path: "/api/partners/one/two", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gotID, gotOK := partnerIDFromPath(tt.path)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotID != tt.wantID {
				t.Fatalf("id = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}
