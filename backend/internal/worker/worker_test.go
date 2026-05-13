package worker

import "testing"

func TestIsPublicWebhookBase(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "empty", raw: "", want: false},
		{name: "localhost", raw: "http://localhost:8080", want: false},
		{name: "loopback ip", raw: "http://127.0.0.1:8080", want: false},
		{name: "private ip", raw: "http://192.168.1.10:8080", want: false},
		{name: "docker host", raw: "http://host.docker.internal:8080", want: false},
		{name: "public domain", raw: "https://api.example.com", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPublicWebhookBase(tt.raw); got != tt.want {
				t.Fatalf("IsPublicWebhookBase(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
