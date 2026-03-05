package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"go-chat/global"
	"go-chat/internal/models"
	"strconv"
	"sync"
	"time"
)

type RelationRepository struct{}

// Local Cache: map[string]relationStatus
// Key: "relation:ownerID:targetID"
// Value: cacheItem{val: "1" or "0", expiresAt: int64}
var localCache sync.Map

// CacheItem with TTL
type cacheItem struct {
	val       string
	expiresAt int64
}

const (
	LocalCacheTTL      = 5 * time.Minute // Short TTL for safety
	RelationEventTopic = "im:relation:event"
)

type RelationEvent struct {
	Action   string `json:"action"` // "del"
	OwnerID  uint   `json:"owner_id"`
	TargetID uint   `json:"target_id"`
}

func NewRelationRepository() *RelationRepository {
	return &RelationRepository{}
}

func (r *RelationRepository) IsFriend(ctx context.Context, userId uint, targetId uint) bool {
	var relation models.Relation
	if err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? AND type = 1", userId, targetId).
		First(&relation).Error; err != nil {
		return false
	}
	if err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ? AND type = 1", targetId, userId).
		First(&relation).Error; err != nil {
		return false
	}
	return true
}

func (r *RelationRepository) GetRelationCacheKey(ownerID, targetID uint) string {
	return fmt.Sprintf("relation:%d:%d", ownerID, targetID)
}

func (r *RelationRepository) IsFriendCached(ctx context.Context, userID uint, targetID uint) bool {
	key1 := r.GetRelationCacheKey(userID, targetID)
	key2 := r.GetRelationCacheKey(targetID, userID)

	// 1. L1: Local Cache Check
	val1, ok1 := r.getLocalCache(key1)
	val2, ok2 := r.getLocalCache(key2)

	if ok1 && ok2 {
		return val1 == "1" && val2 == "1"
	}

	// 2. L2: Redis Cache (Pipeline)
	pipe := global.RDB.Pipeline()
	cmd1 := pipe.Get(ctx, key1)
	cmd2 := pipe.Get(ctx, key2)
	_, _ = pipe.Exec(ctx)

	// Check key1
	redisVal1, err1 := cmd1.Result()
	if err1 != nil {
		// Cache miss -> load from DB
		redisVal1 = r.loadRelationToCache(ctx, userID, targetID)
	}
	// Update L1
	r.setLocalCache(key1, redisVal1)

	if redisVal1 != "1" {
		return false
	}

	// Check key2
	redisVal2, err2 := cmd2.Result()
	if err2 != nil {
		redisVal2 = r.loadRelationToCache(ctx, targetID, userID)
	}
	// Update L1
	r.setLocalCache(key2, redisVal2)

	if redisVal2 != "1" {
		return false
	}

	return true
}

func (r *RelationRepository) getLocalCache(key string) (string, bool) {
	if v, ok := localCache.Load(key); ok {
		item := v.(cacheItem)
		if time.Now().Unix() > item.expiresAt {
			localCache.Delete(key)
			return "", false
		}
		return item.val, true
	}
	return "", false
}

func (r *RelationRepository) setLocalCache(key string, val string) {
	localCache.Store(key, cacheItem{
		val:       val,
		expiresAt: time.Now().Add(LocalCacheTTL).Unix(),
	})
}

func (r *RelationRepository) InvalidateRelationCache(ctx context.Context, ownerID, targetID uint) {
	// 1. Del Redis (L2)
	key1 := r.GetRelationCacheKey(ownerID, targetID)
	key2 := r.GetRelationCacheKey(targetID, ownerID)
	global.RDB.Del(ctx, key1, key2)

	// 2. Publish Event to invalidate Local Cache on all nodes (L1)
	event := RelationEvent{
		Action:   "del",
		OwnerID:  ownerID,
		TargetID: targetID,
	}
	bytes, _ := json.Marshal(event)
	global.RDB.Publish(ctx, RelationEventTopic, bytes)
}

// StartRelationEventListener starts a goroutine to listen for cache invalidation events
func StartRelationEventListener() {
	go func() {
		global.Log.Info("Starting Relation Cache Invalidation Listener...")
		pubsub := global.RDB.Subscribe(context.Background(), RelationEventTopic)
		defer pubsub.Close()
		ch := pubsub.Channel()

		repo := &RelationRepository{}

		for msg := range ch {
			var event RelationEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}

			if event.Action == "del" {
				// Invalidate both directions locally
				k1 := repo.GetRelationCacheKey(event.OwnerID, event.TargetID)
				k2 := repo.GetRelationCacheKey(event.TargetID, event.OwnerID)
				localCache.Delete(k1)
				localCache.Delete(k2)
				// global.Log.Debug("Invalidated local cache", zap.String("k1", k1))
			}
		}
	}()
}

func (r *RelationRepository) loadRelationToCache(ctx context.Context, ownerID, targetID uint) string {
	var relation models.Relation
	err := global.DB.WithContext(ctx).
		Where("owner_id = ? AND target_id = ?", ownerID, targetID).
		First(&relation).Error

	val := ""
	if err != nil {
		// No relation found
		val = "0"
	} else {
		val = strconv.Itoa(relation.Type)
	}

	// Set cache (e.g., 24h)
	key := r.GetRelationCacheKey(ownerID, targetID)
	global.RDB.Set(ctx, key, val, 24*time.Hour)

	return val
}
