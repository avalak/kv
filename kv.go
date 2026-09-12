// Package kv provides a generic, TTL-aware cache with pluggable backends.
//
// TTL is always supplied by the caller at Save time; the cache does not
// inspect the value. This keeps a single interface usable for HTTP responses,
// DNS messages and arbitrary domain types.
package kv

import "errors"

// ErrNotFound is returned by Backend.Load on a miss or expired entry.
var ErrNotFound = errors.New("kv: not found")
