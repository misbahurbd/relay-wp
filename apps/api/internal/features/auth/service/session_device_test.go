package service

import (
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
)

func TestDescribeDevice(t *testing.T) {
	tests := []struct {
		name, client, ua, header string
		wantName, wantKind       string
	}{
		{
			"chrome mac", model.ClientWeb,
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
			"", "Chrome on macOS", deviceDesktop,
		},
		{
			"edge windows", model.ClientWeb,
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 Edg/130.0",
			"", "Edge on Windows", deviceDesktop,
		},
		{
			"safari iphone", model.ClientWeb,
			"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
			"", "Safari on iPhone", deviceMobile,
		},
		{
			"safari ipad", model.ClientWeb,
			"Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
			"", "Safari on iPad", deviceTablet,
		},
		{
			"chrome android phone", model.ClientWeb,
			"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36",
			"", "Chrome on Android", deviceMobile,
		},
		{
			"android tablet", model.ClientWeb,
			"Mozilla/5.0 (Linux; Android 15; SM-X910) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
			"", "Chrome on Android", deviceTablet,
		},
		{
			"firefox linux", model.ClientWeb,
			"Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0",
			"", "Firefox on Linux", deviceDesktop,
		},
		{"unknown", model.ClientWeb, "curl/8.7.1", "", "Unknown device", deviceDesktop},
		{"app with header", model.ClientMobile, "okhttp/4", "iPhone 15", "Relay app on iPhone 15", deviceApp},
		{"app without header", model.ClientMobile, "okhttp/4", "", "Relay app", deviceApp},
		{"app header cleaned", model.ClientMobile, "", "  Pixel\x00 9\n ", "Relay app on Pixel 9", deviceApp},
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
	name, _ := describeDevice(model.ClientMobile, "", strings.Repeat("x", 500))
	if got := len([]rune(strings.TrimPrefix(name, "Relay app on "))); got != maxDeviceNameLen {
		t.Errorf("device name length = %d, want %d", got, maxDeviceNameLen)
	}
}
