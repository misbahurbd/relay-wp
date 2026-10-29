package service

import (
	"strings"
	"unicode"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
)

// Device types stored on a session; they pick the icon in the UI.
const (
	deviceDesktop = "desktop"
	deviceMobile  = "mobile"
	deviceTablet  = "tablet"
	deviceApp     = "app"
)

const maxDeviceNameLen = 100

// describeDevice turns a request into the name and type shown on the sessions page.
// The mobile app sends X-Device-Name; browsers are described from the user agent.
func describeDevice(client, userAgent, deviceHeader string) (name, kind string) {
	if client == model.ClientMobile {
		if n := cleanDeviceName(deviceHeader); n != "" {
			return "Relay app on " + n, deviceApp
		}
		return "Relay app", deviceApp
	}

	browser, os, kind := parseUserAgent(userAgent)
	switch {
	case browser != "" && os != "":
		return browser + " on " + os, kind
	case browser != "":
		return browser, kind
	case os != "":
		return os, kind
	default:
		return "Unknown device", deviceDesktop
	}
}

// cleanDeviceName makes a client-supplied name safe to store and show.
func cleanDeviceName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxDeviceNameLen {
		s = string(r[:maxDeviceNameLen])
	}
	return s
}

func parseUserAgent(ua string) (browser, os, kind string) {
	has := func(s string) bool { return strings.Contains(ua, s) }

	switch {
	case has("Edg/") || has("EdgiOS/") || has("EdgA/"):
		browser = "Edge"
	case has("OPR/") || has("Opera"):
		browser = "Opera"
	case has("Firefox/") || has("FxiOS/"):
		browser = "Firefox"
	case has("SamsungBrowser/"):
		browser = "Samsung Internet"
	case has("CriOS/") || has("Chrome/"):
		browser = "Chrome"
	case has("Safari/"):
		browser = "Safari"
	}

	kind = deviceDesktop
	switch {
	case has("iPad"):
		os, kind = "iPad", deviceTablet
	case has("iPhone"):
		os, kind = "iPhone", deviceMobile
	case has("Android"):
		os, kind = "Android", deviceTablet
		if has("Mobile") {
			kind = deviceMobile
		}
	case has("Windows"):
		os = "Windows"
	case has("CrOS"):
		os = "ChromeOS"
	case has("Macintosh") || has("Mac OS X"):
		os = "macOS"
	case has("Linux"):
		os = "Linux"
	}
	return browser, os, kind
}
