package types

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	req := Request{Path: `C:\Temp\test.exe`}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Path != req.Path {
		t.Errorf("expected path %q, got %q", req.Path, decoded.Path)
	}
}

func TestResponseMarshal(t *testing.T) {
	tests := []struct {
		resp    Response
		wantErr bool
	}{
		{Response{Result: "Allow", Message: "Elevation Successful"}, false},
		{Response{Result: "Deny", Message: "Application Not Approved"}, false},
		{Response{Result: "Error", Message: "Service Not Running"}, false},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.resp)
		if err != nil {
			t.Fatalf("Marshal(%+v) error: %v", tt.resp, err)
		}

		var decoded Response
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if decoded.Result != tt.resp.Result {
			t.Errorf("expected Result %q, got %q", tt.resp.Result, decoded.Result)
		}
		if decoded.Message != tt.resp.Message {
			t.Errorf("expected Message %q, got %q", tt.resp.Message, decoded.Message)
		}
	}
}
