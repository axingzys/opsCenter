package asset

import "testing"

func TestSanitizeAgentBaseURLUsesRequestHostForLocalDefaults(t *testing.T) {
	tests := []struct {
		name         string
		rawBaseURL   string
		fallbackHost string
		defaultPort  int
		want         string
	}{
		{
			name:         "frontend host without port stays on frontend",
			rawBaseURL:   "http://localhost:9876",
			fallbackHost: "192.168.1.12",
			defaultPort:  9876,
			want:         "http://192.168.1.12",
		},
		{
			name:         "explicit request port is preserved",
			rawBaseURL:   "http://localhost:9876",
			fallbackHost: "192.168.1.12:8080",
			defaultPort:  9876,
			want:         "http://192.168.1.12:8080",
		},
		{
			name:         "configured public url is left unchanged",
			rawBaseURL:   "https://opshub.example.com",
			fallbackHost: "192.168.1.12",
			defaultPort:  9876,
			want:         "https://opshub.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeAgentBaseURL(tt.rawBaseURL, tt.fallbackHost, tt.defaultPort); got != tt.want {
				t.Fatalf("sanitizeAgentBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
