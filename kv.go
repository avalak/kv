// Package kv provides a generic, TTL-aware cache with pluggable backends.
//
// TTL is always supplied by the caller at Save time; the cache does not
// inspect the value. This keeps a single interface usable for HTTP responses,
// DNS messages and arbitrary domain types.
package kv

import (
	"errors"
	"math"
	"time"
)

// ErrNotFound is returned by Backend.Load on a miss or expired entry.
var ErrNotFound = errors.New("kv: not found")

// ErrPrefixUnsupported is returned by Cache.DeletePrefix and by a
// backend's DropPrefix when the backend cannot match keys by prefix.
var ErrPrefixUnsupported = errors.New("kv: prefix not supported")

// NoExpiry is the TTL sentinel that stores an entry without an expiry.
// Any other ttl <= 0 means "do not store".
const NoExpiry time.Duration = math.MinInt64
