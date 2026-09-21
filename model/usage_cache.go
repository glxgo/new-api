package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// usageCacheSchema is bumped whenever the serialized projection contract or
// aggregation semantics change. Keeping it in the key makes a rolling local
// build safe when Redis is shared by old and new application instances.
const usageCacheSchema = "usage-projection-v1"

// usageProjectionSharedQueryTimeout bounds work owned by a singleflight
// leader.  The leader must not inherit an HTTP request's cancellation: a
// browser tab closing (or a reverse proxy timing out one waiter) should not
// cancel the database read for every other waiter sharing the same key.  This
// is deliberately a generous internal budget rather than the old five-second
// UI timeout; callers still retain their own request deadlines while waiting.
const usageProjectionSharedQueryTimeout = 2 * time.Minute

// usageProjectionWorkContext keeps context values (for tracing/tenant
// propagation) but removes cancellation and deadlines from the initiating
// request.  The shared operation receives its own finite deadline instead.
func usageProjectionWorkContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(parent), usageProjectionSharedQueryTimeout)
}

// waitUsageProjectionResult lets a waiter stop waiting when its own request
// is gone, while the singleflight leader continues on its independent work
// context. DoChan is important here; Group.Do would make the waiter inherit
// the leader's lifetime with no way to honor its local cancellation.
func waitUsageProjectionResult(ctx context.Context, resultCh <-chan singleflight.Result) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case result := <-resultCh:
		return result.Val, result.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func usageCacheTenant() string {
	tenant := strings.TrimSpace(os.Getenv("USAGE_CACHE_TENANT"))
	if tenant == "" {
		tenant = "default"
	}
	return tenant
}

// usageCacheDatabaseNamespace separates cache entries belonging to different
// logical log databases without putting a DSN (which may contain credentials)
// into a key.  The dialector's DSN is stable across application processes,
// while a raw *gorm.DB pointer is not; using only the pointer would make
// cross-instance Redis sharing impossible and can also collide in tests after
// a database is closed and recreated.
func usageCacheDatabaseNamespace(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return "none"
	}
	name := db.Dialector.Name()
	dsn := ""
	value := reflect.ValueOf(db.Dialector)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			break
		}
		value = value.Elem()
	}
	if value.IsValid() && value.Kind() == reflect.Struct {
		// DSN is either a direct field (sqlite) or promoted from the embedded
		// Config (mysql/postgres). FieldByName follows the promoted path.
		field := value.FieldByName("DSN")
		if field.IsValid() && field.Kind() == reflect.String && field.CanInterface() {
			dsn = field.String()
		}
	}
	identity := name + "\x00" + dsn
	if dsn == "" {
		// Custom dialectors may not expose a DSN. Include their concrete type
		// so unrelated drivers do not share a namespace; the explicit tenant
		// remains available for operators that need stronger isolation.
		identity = fmt.Sprintf("%s\x00%T", name, db.Dialector)
	}
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:8])
}

func usageCacheLocalKey(remoteKey string) string {
	return usageCacheDatabaseNamespace(LOG_DB) + "|" + remoteKey
}

// UsageProjectionCacheKey exposes the versioned, database-scoped key builder
// to sibling packages that cache an aggregate type owned outside model (for
// example the dashboard service).  It intentionally keeps the DSN out of the
// key and preserves the same tenant/schema namespace as the model readers.
func UsageProjectionCacheKey(scope string, userID int, start, end, bucket int64, timezone, filter string) string {
	return usageCacheKey(scope, userID, start, end, bucket, timezone, filter)
}

// UsageProjectionCacheLocalKey returns the process-local namespace for a
// shared projection key.  The local prefix prevents tests or multiple logical
// log databases in one process from accidentally sharing hot entries.
func UsageProjectionCacheLocalKey(remoteKey string) string {
	return usageCacheLocalKey(remoteKey)
}

func usageCacheKey(scope string, userID int, start, end, bucket int64, timezone, filter string) string {
	// The administrator endpoint resolves usernames to immutable user IDs
	// before reaching this layer. Never put a mutable username in a cache key.
	return fmt.Sprintf("%s:%s:%s:%d:%d:%d:%d:%s:%s:%s", usageCacheSchema, usageCacheTenant(), usageCacheDatabaseNamespace(LOG_DB), userID, start, end, bucket, scope, timezone, filter)
}

// normalizeHotUsageWindow keeps the cache identity exact.  Earlier versions
// quantized the moving end timestamp so requests arriving a few seconds apart
// shared an entry, but the query itself still used the caller's unquantized
// range; whichever request won the race could therefore return rows from a
// subtly different interval.  The short TTL already bounds hot-dashboard
// churn, while exact keys preserve the half-open range contract.
func normalizeHotUsageWindow(start, end int64) (int64, int64) {
	return start, end
}

func usageCacheGet(ctx context.Context, key string, destination any) (bool, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := common.RDB.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := common.Unmarshal(payload, destination); err != nil {
		// A malformed value must not poison the key forever. Delete is best
		// effort; the caller still falls back to the database/local cache.
		_ = common.RDB.Del(ctx, key).Err()
		return false, err
	}
	return true, nil
}

// UsageProjectionCacheGet is the read-side wrapper for aggregate caches owned
// by sibling packages. Redis failures remain non-fatal to callers, which can
// fall back to their local cache or database path.
func UsageProjectionCacheGet(ctx context.Context, key string, destination any) (bool, error) {
	return usageCacheGet(ctx, key, destination)
}

func usageCacheSet(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := common.Marshal(value)
	if err != nil {
		return err
	}
	return common.RDB.Set(ctx, key, payload, ttl).Err()
}

// UsageProjectionCacheSet is the write-side wrapper for aggregate caches owned
// by sibling packages.
func UsageProjectionCacheSet(ctx context.Context, key string, value any, ttl time.Duration) error {
	return usageCacheSet(ctx, key, value, ttl)
}
