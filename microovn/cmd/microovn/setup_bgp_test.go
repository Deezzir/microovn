package main

import (
	"testing"
)

func TestParseSetupAsnRange(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		wantMin   uint64
		wantMax   uint64
		wantError bool
	}{
		{
			name:    "valid range",
			input:   "4200000000-4294967294",
			wantMin: 4200000000,
			wantMax: 4294967294,
		},
		{
			name:      "missing dash",
			input:     "4200000000",
			wantError: true,
		},
		{
			name:      "too many dashes",
			input:     "4200000000-4294967294-100",
			wantError: true,
		},
		{
			name:      "invalid min",
			input:     "abc-4294967294",
			wantError: true,
		},
		{
			name:      "invalid max",
			input:     "4200000000-abc",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseSetupAsnRange(tc.input)
			if tc.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result[0] != tc.wantMin {
				t.Errorf("expected min %d, got %d", tc.wantMin, result[0])
			}
			if result[1] != tc.wantMax {
				t.Errorf("expected max %d, got %d", tc.wantMax, result[1])
			}
		})
	}
}
