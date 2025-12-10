package libyaci

import (
	"testing"
)

func TestParseMethodFullName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantService string
		wantMethod  string
		wantErr     bool
	}{
		{
			name:        "valid cosmos method",
			input:       "cosmos.bank.v1beta1.Query.Balance",
			wantService: "cosmos.bank.v1beta1.Query",
			wantMethod:  "Balance",
			wantErr:     false,
		},
		{
			name:        "valid simple method",
			input:       "Service.Method",
			wantService: "Service",
			wantMethod:  "Method",
			wantErr:     false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no dot",
			input:   "NoServiceMethod",
			wantErr: true,
		},
		{
			name:    "trailing dot",
			input:   "Service.",
			wantErr: true,
		},
		{
			name:    "leading dot",
			input:   ".Method",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, method, err := parseMethodFullName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if service != tt.wantService {
				t.Errorf("service = %q, want %q", service, tt.wantService)
			}
			if method != tt.wantMethod {
				t.Errorf("method = %q, want %q", method, tt.wantMethod)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	if opts.insecure {
		t.Error("insecure should default to false")
	}
	if opts.maxRetries != defaultMaxRetries {
		t.Errorf("maxRetries = %d, want %d", opts.maxRetries, defaultMaxRetries)
	}
	if opts.maxRecvMsgSize != defaultMaxRecvMsgSize {
		t.Errorf("maxRecvMsgSize = %d, want %d", opts.maxRecvMsgSize, defaultMaxRecvMsgSize)
	}
}

func TestWithOptions(t *testing.T) {
	opts := defaultOptions()

	WithInsecure()(opts)
	if !opts.insecure {
		t.Error("WithInsecure should set insecure to true")
	}

	WithMaxRetries(10)(opts)
	if opts.maxRetries != 10 {
		t.Errorf("WithMaxRetries: got %d, want 10", opts.maxRetries)
	}

	WithMaxRecvMsgSize(8 * 1024 * 1024)(opts)
	if opts.maxRecvMsgSize != 8*1024*1024 {
		t.Errorf("WithMaxRecvMsgSize: got %d, want %d", opts.maxRecvMsgSize, 8*1024*1024)
	}
}
