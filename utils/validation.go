package utils

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func NormalizePort(value float64, defaultPort int, field string) (int, error) {
	if value == 0 {
		return defaultPort, nil
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value {
		return 0, fmt.Errorf("%s must be an integer port", field)
	}
	port := int(value)
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", field)
	}
	return port, nil
}

func SelectExactVersion(input string, versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions available")
	}
	if input == "" || input == "latest" {
		return versions[0], nil
	}
	for _, version := range versions {
		if version == input {
			return version, nil
		}
	}
	return "", fmt.Errorf("version %q not found", input)
}

func ValidateProxyAddress(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid proxy_address: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("proxy_address scheme must be http or https")
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("proxy_address must include a host")
	}
	return u.String(), nil
}

func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	if strings.ContainsAny(domain, "/\\:@ \t\r\n") {
		return fmt.Errorf("domain must be a hostname without scheme, path, port, or whitespace")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return fmt.Errorf("domain must be a fully qualified hostname")
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return fmt.Errorf("domain contains an invalid label")
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("domain labels must not start or end with '-'")
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return fmt.Errorf("domain contains unsupported characters")
		}
	}
	return nil
}

func ParseUintID(raw string) (uint, bool) {
	if raw == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}
