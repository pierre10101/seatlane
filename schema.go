// Package seatlane holds the app's module-level assets.
package seatlane

import _ "embed"

// Schema is the canonical schema (schema.sql). sqlc reads the same file.
//
//go:embed schema.sql
var Schema string
