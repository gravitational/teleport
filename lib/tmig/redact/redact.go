// Package redact provides utilities for scrubbing secrets from Teleport
// configuration text and arbitrary strings before they are included in reports.
package redact

import "regexp"

var (
	authTokenRe     = regexp.MustCompile(`(?m)(auth_token:\s*)"?([^"\s\n]+)"?`)
	regSecretRe     = regexp.MustCompile(`(?m)(registration_secret:\s*)"?([^"\s\n]+)"?`)
	bearerKeyRe     = regexp.MustCompile(`(?i)(bearer[_-]?token[^:]*:\s*)"?([^"\s\n]+)"?`)
	bearerValRe     = regexp.MustCompile(`(?i)(\S+:\s*)"?(bearer[_-][^\s"\n]+)"?`)
	genericSecretRe = regexp.MustCompile(`(?m)(secret:\s*)"?([^"\s\n]+)"?`)
)

const redacted = "<redacted>"

// Config redacts known secret patterns in teleport config text.
// Fields like token_name (a reference, not a secret) are preserved.
func Config(input string) string {
	result := authTokenRe.ReplaceAllString(input, "${1}"+redacted)
	result = regSecretRe.ReplaceAllString(result, "${1}"+redacted)
	result = genericSecretRe.ReplaceAllString(result, "${1}"+redacted)
	return result
}

// Secrets redacts all known secret patterns in arbitrary text.
// It applies Config patterns plus additional patterns like bearer tokens.
func Secrets(input string) string {
	result := Config(input)
	result = bearerKeyRe.ReplaceAllString(result, "${1}"+redacted)
	result = bearerValRe.ReplaceAllString(result, "${1}"+redacted)
	return result
}
