// Package locale localizes safe response text without changing tool execution.
package locale

import (
	"context"
	"math"
	"os"
	"strconv"
	"strings"
)

type Language string

const (
	English Language = "en"
	Chinese Language = "zh-CN"
)

type contextKey struct{}
type selection struct {
	language Language
	http     bool
}

func Current(ctx context.Context) Language {
	if value, ok := ctx.Value(contextKey{}).(selection); ok && value.language == Chinese {
		return Chinese
	}
	return English
}

func WithLanguage(ctx context.Context, language Language) context.Context {
	if language != Chinese {
		language = English
	}
	return context.WithValue(ctx, contextKey{}, selection{language: language})
}

// Environment is read when constructing an adapter, never stored globally.
// MCP_LANGUAGE is an explicit default; otherwise use the standard message locale.
func Environment() Language {
	for _, name := range []string{"MCP_LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			language, _ := languageTag(value)
			return language
		}
	}
	return English
}

// RequestContext always replaces a session's previous choice. The current HTTP
// request wins over optional per-call _meta.locale, then the adapter default.
func RequestContext(ctx context.Context, acceptLanguage string, meta map[string]any, fallback Language) context.Context {
	ctx = WithLanguage(ctx, fallback)
	if strings.TrimSpace(acceptLanguage) != "" {
		return context.WithValue(ctx, contextKey{}, selection{language: acceptedLanguage(acceptLanguage), http: true})
	}
	return WithMetadata(ctx, meta)
}

// WithMetadata handles this project's optional MCP _meta.locale convention.
// It is separate from tool arguments and cannot override an HTTP preference.
func WithMetadata(ctx context.Context, meta map[string]any) context.Context {
	if value, ok := ctx.Value(contextKey{}).(selection); ok && value.http {
		return ctx
	}
	if value, ok := meta["locale"].(string); ok && strings.TrimSpace(value) != "" {
		language, _ := languageTag(value)
		return WithLanguage(ctx, language)
	}
	return ctx
}

func languageTag(value string) (Language, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.SplitN(strings.SplitN(value, ".", 2)[0], "@", 2)[0]
	value = strings.ReplaceAll(value, "_", "-")
	switch {
	case value == "zh" || strings.HasPrefix(value, "zh-"):
		return Chinese, true
	case value == "en" || strings.HasPrefix(value, "en-"):
		return English, true
	default:
		return English, false
	}
}

func acceptedLanguage(header string) Language {
	if len(header) > 4096 {
		return English
	}
	best, quality := English, -1.0
	for _, preference := range strings.Split(header, ",") {
		parts := strings.Split(preference, ";")
		language, supported := languageTag(parts[0])
		if !supported {
			continue
		}
		q, valid, hasQuality := 1.0, true, false
		for _, parameter := range parts[1:] {
			name, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
				valid = false
				break
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if hasQuality || err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 || parsed > 1 {
				valid = false
				break
			}
			q, hasQuality = parsed, true
		}
		if valid && q > 0 && q > quality {
			best, quality = language, q
		}
	}
	return best
}
