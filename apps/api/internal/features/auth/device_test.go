package auth

import (
	"strings"
	"testing"
)

func TestDescribeDevice(t *testing.T) {
	tests := []struct {
		name, client, ua, header string
		wantName, wantKind       string
	}{
		{
			"chrome mac", clientWeb,
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
			"", "Chrome on macOS", deviceDesktop,
		},
		{
			"edge windows", clientWeb,
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 Edg/130.0",
			"", "Edge on Windows", deviceDesktop,
		},
		{
			"safari iphone", clientWeb,
			"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
			"", "Safari on iPhone", deviceMobile,
		},
		{
			"safari ipad", clientWeb,
			"Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
			"", "Safari on iPad", deviceTablet,
		},
		{
			"chrome android phone", clientWeb,
			"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36",
			"", "Chrome on Android", deviceMobile,
		},
		{
			"android tablet", clientWeb,
			"Mozilla/5.0 (Linux; Android 15; SM-X910) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
			"", "Chrome on Android", deviceTablet,
		},
		{
			"firefox linux", clientWeb,
			"Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0",
			"", "Firefox on Linux", deviceDesktop,
		},
		{"unknown", clientWeb, "curl/8.7.1", "", "Unknown device", deviceDesktop},
		{"app with header", clientMobile, "okhttp/4", "iPhone 15", "Relay app on iPhone 15", deviceApp},
		{"app without header", clientMobile, "okhttp/4", "", "Relay app", deviceApp},
		{"app header cleaned", clientMobile, "", "  Pixel\x00 9\n ", "Relay app on Pixel 9", deviceApp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, kind := describeDevice(tt.client, tt.ua, tt.header)
			if name != tt.wantName || kind != tt.wantKind {
				t.Errorf("got (%q, %q), want (%q, %q)", name, kind, tt.wantName, tt.wantKind)
			}
		})
	}
}

func TestDeviceNameIsCapped(t *testing.T) {
	name, _ := describeDevice(clientMobile, "", strings.Repeat("x", 500))
	if got := len([]rune(strings.TrimPrefix(name, "Relay app on "))); got != maxDeviceNameLen {
		t.Errorf("device name length = %d, want %d", got, maxDeviceNameLen)
	}
}
