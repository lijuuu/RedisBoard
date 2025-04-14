package redisboard

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)


// Package redisboard provides a Redis-based ranking system with support for:
// - global rankings across all users
// - entity-based rankings (e.g., by country)
// - configurable top-k listings
// - atomic score updates
// - float or integer scores
// Uses Redis sorted sets (ZSET) for O(log n) ranking operations.

// Config defines settings for leaderboard initialization.
type Config struct {
	Namespace   string // prefix for redis keys (e.g., "game1")
	K           int    // number of top users to track (e.g., 10)
	MaxUsers    int    // maximum allowed users (e.g., 1M)
	MaxEntities int    // maximum allowed entities (e.g., 200)
	FloatScores bool   // true: keep decimals, false: round to integers
	RedisAddr   string // redis connection address (e.g., "localhost:6379")
	RedisPass   string // optional redis authentication
}

// User represents a single leaderboard entry with score and grouping.
type User struct {
	ID     string  // unique user identifier
	Entity string  // grouping key (e.g., country code)
	Score  float64 // current score (rounded if FloatScores=false)
}

// LeaderboardData holds complete ranking information for a user.
type LeaderboardData struct {
	UserID     string  `json:"userID"`     // user identifier
	Score      float64 `json:"score"`      // current score
	Entity     string  `json:"entity"`     // grouping identifier
	GlobalRank int     `json:"globalRank"` // position across all users (0-based)
	EntityRank int     `json:"entityRank"` // position within entity (0-based)
	TopKGlobal []User  `json:"topKGlobal"` // top k users globally
	TopKEntity []User  `json:"topKEntity"` // top k users in same entity
}

// Leaderboard manages the ranking system using Redis backend.
type Leaderboard struct {
	config Config          // configuration settings
	client *redis.Client   // redis connection
	ctx    context.Context // context for redis operations
}

// Redis key structure:
// {namespace}:global         -> zset of all users and scores
// {namespace}:user:entities  -> hash mapping users to entities
// {namespace}:entity:{code}  -> zset of users/scores per entity

// New creates leaderboard instance with given config.
// Validates config values and sets defaults if needed:
// - Namespace: "default" if empty
// - K: 10 if <= 0
// - MaxUsers: 1M if <= 0
// - MaxEntities: 200 if <= 0
// - RedisAddr: "localhost:6379" if empty
// Returns error if Redis connection fails.
func New(cfg Config) (*Leaderboard, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.K <= 0 {
		cfg.K = 10
	}
	if cfg.MaxUsers <= 0 {
		cfg.MaxUsers = 1_000_000
	}
	if cfg.MaxEntities <= 0 {
		cfg.MaxEntities = 200
	}
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	ctx := context.Background()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Leaderboard{
		config: cfg,
		client: client,
		ctx:    ctx,
	}, nil
}

// Close properly shuts down Redis connection.
// Should be called when leaderboard is no longer needed.
func (lb *Leaderboard) Close() error {
	return lb.client.Close()
}

// AddUser creates or updates user score in rankings.
// Updates both global and entity-specific rankings.
// Uses atomic operations via Redis pipeline.
// Returns error if:
// - user ID is empty
// - score is negative
// - Redis operation fails
func (lb *Leaderboard) AddUser(user User) error {
	if user.ID == "" || user.Score < 0 {
		return fmt.Errorf("invalid user ID or score")
	}

	score := user.Score
	if !lb.config.FloatScores {
		score = float64(int(score))
	}

	globalKey := lb.config.Namespace + ":global"
	entitiesKey := lb.config.Namespace + ":user:entities"
	entityKey := lb.config.Namespace + ":entity:" + user.Entity

	pipe := lb.client.Pipeline()
	pipe.ZAdd(lb.ctx, globalKey, redis.Z{Score: score, Member: user.ID})
	pipe.HSet(lb.ctx, entitiesKey, user.ID, user.Entity)
	if user.Entity != "" {
		pipe.ZAdd(lb.ctx, entityKey, redis.Z{Score: score, Member: user.ID})
	}
	_, err := pipe.Exec(lb.ctx)
	if err != nil {
		return fmt.Errorf("failed to add user: %w", err)
	}
	return nil
}

// IncrementScore adds to user's current score.
// Updates both global and entity rankings atomically.
// Returns error if:
// - user ID is empty
// - increment is zero
// - Redis operation fails
func (lb *Leaderboard) IncrementScore(userID, entity string, scoreIncrement float64) error {
	if userID == "" || scoreIncrement == 0 {
		return fmt.Errorf("invalid user ID or score increment")
	}

	if !lb.config.FloatScores {
		scoreIncrement = float64(int(scoreIncrement))
	}

	globalKey := lb.config.Namespace + ":global"
	entitiesKey := lb.config.Namespace + ":user:entities"
	entityKey := lb.config.Namespace + ":entity:" + entity

	pipe := lb.client.Pipeline()
	pipe.ZIncrBy(lb.ctx, globalKey, scoreIncrement, userID)
	pipe.HSet(lb.ctx, entitiesKey, userID, entity) // Always update
	if entity != "" {
		pipe.ZIncrBy(lb.ctx, entityKey, scoreIncrement, userID)
	}
	_, err := pipe.Exec(lb.ctx)
	if err != nil {
		return fmt.Errorf("failed to increment score: %w", err)
	}
	return nil
}

// RemoveUser deletes user from all rankings.
// Removes from global ranking and entity ranking.
// Cleans up entity mapping.
// Returns error if:
// - user ID is empty
// - Redis operation fails
func (lb *Leaderboard) RemoveUser(userID string) error {
	if userID == "" {
		return fmt.Errorf("invalid user ID")
	}

	entitiesKey := lb.config.Namespace + ":user:entities"
	globalKey := lb.config.Namespace + ":global"

	entity, err := lb.client.HGet(lb.ctx, entitiesKey, userID).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get user entity: %w", err)
	}

	pipe := lb.client.Pipeline()
	pipe.ZRem(lb.ctx, globalKey, userID)
	pipe.HDel(lb.ctx, entitiesKey, userID)
	if entity != "" {
		entityKey := lb.config.Namespace + ":entity:" + entity
		pipe.ZRem(lb.ctx, entityKey, userID)
	}
	_, err = pipe.Exec(lb.ctx)
	if err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}
	return nil
}

// UpdateEntityByUserID changes a user's entity, updating rankings atomically.
// Removes the user from the old entity's sorted set, adds to the new entity's
// sorted set with the same score, and updates the entity mapping.
// Returns error if:
// - userID is empty
// - user doesn't exist
// - newEntity is empty
// - Redis operation fails
func (lb *Leaderboard) UpdateEntityByUserID(userID, newEntity string) error {
	if userID == "" {
		return fmt.Errorf("invalid user ID")
	}
	if newEntity == "" {
		return fmt.Errorf("invalid new entity")
	}

	globalKey := lb.config.Namespace + ":global"
	entitiesKey := lb.config.Namespace + ":user:entities"

	// Check if user exists and get current entity
	pipe := lb.client.Pipeline()
	scoreCmd := pipe.ZScore(lb.ctx, globalKey, userID)
	entityCmd := pipe.HGet(lb.ctx, entitiesKey, userID)
	_, err := pipe.Exec(lb.ctx)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to fetch user data: %w", err)
	}
	if scoreCmd.Err() == redis.Nil {
		return fmt.Errorf("user %s not found", userID)
	}
	if scoreCmd.Err() != nil {
		return fmt.Errorf("failed to get user score: %w", scoreCmd.Err())
	}
	oldEntity := entityCmd.Val()
	score := scoreCmd.Val()

	// Round score if FloatScores=false
	if !lb.config.FloatScores {
		score = float64(int(score))
	}

	// Update entity and rankings
	newEntityKey := lb.config.Namespace + ":entity:" + newEntity
	pipe = lb.client.Pipeline()
	pipe.HSet(lb.ctx, entitiesKey, userID, newEntity)
	pipe.ZAdd(lb.ctx, newEntityKey, redis.Z{Score: score, Member: userID})
	if oldEntity != "" && oldEntity != newEntity {
		oldEntityKey := lb.config.Namespace + ":entity:" + oldEntity
		pipe.ZRem(lb.ctx, oldEntityKey, userID)
	}
	_, err = pipe.Exec(lb.ctx)
	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}
	return nil
}

// GetUserLeaderboardData fetches complete ranking data.
// Includes:
// - current score
// - global and entity ranks
// - top k users globally
// - top k users in same entity
// Returns error if Redis operations fail.
func (lb *Leaderboard) GetUserLeaderboardData(userID string) (LeaderboardData, error) {
	globalKey := lb.config.Namespace + ":global"
	entitiesKey := lb.config.Namespace + ":user:entities"

	// Pipeline all Redis queries
	pipe := lb.client.Pipeline()
	globalRankCmd := pipe.ZRevRank(lb.ctx, globalKey, userID)
	entityCmd := pipe.HGet(lb.ctx, entitiesKey, userID)
	scoreCmd := pipe.ZScore(lb.ctx, globalKey, userID)
	topKGlobalCmd := pipe.ZRevRangeWithScores(lb.ctx, globalKey, 0, int64(lb.config.K-1))
	var entityRankCmd *redis.IntCmd
	var topKEntityCmd *redis.ZSliceCmd
	_, err := pipe.Exec(lb.ctx)
	if err != nil && err != redis.Nil {
		return LeaderboardData{}, fmt.Errorf("failed to fetch leaderboard data: %w", err)
	}

	// Process results
	data := LeaderboardData{UserID: userID}
	if globalRankCmd.Err() == redis.Nil {
		data.GlobalRank = -1
	} else if globalRankCmd.Err() != nil {
		return LeaderboardData{}, fmt.Errorf("failed to get global rank: %w", globalRankCmd.Err())
	} else {
		data.GlobalRank = int(globalRankCmd.Val())
	}

	data.Entity = entityCmd.Val()
	if scoreCmd.Err() == redis.Nil {
		data.Score = 0
	} else if scoreCmd.Err() != nil {
		return LeaderboardData{}, fmt.Errorf("failed to get score: %w", scoreCmd.Err())
	} else {
		data.Score = scoreCmd.Val()
	}

	// Top-k global
	if topKGlobalCmd.Err() != nil {
		return LeaderboardData{}, fmt.Errorf("failed to fetch top-k global: %w", topKGlobalCmd.Err())
	}
	pipe = lb.client.Pipeline()
	entityCmds := make(map[string]*redis.StringCmd)
	for _, m := range topKGlobalCmd.Val() {
		userID := m.Member.(string)
		entityCmds[userID] = pipe.HGet(lb.ctx, entitiesKey, userID)
	}
	_, err = pipe.Exec(lb.ctx)
	if err != nil && err != redis.Nil {
		return LeaderboardData{}, fmt.Errorf("failed to fetch top-k entities: %w", err)
	}
	for _, m := range topKGlobalCmd.Val() {
		userID := m.Member.(string)
		data.TopKGlobal = append(data.TopKGlobal, User{
			ID:     userID,
			Entity: entityCmds[userID].Val(),
			Score:  m.Score,
		})
	}

	// Entity data if applicable
	if data.Entity != "" {
		entityKey := lb.config.Namespace + ":entity:" + data.Entity
		pipe = lb.client.Pipeline()
		entityRankCmd = pipe.ZRevRank(lb.ctx, entityKey, userID)
		topKEntityCmd = pipe.ZRevRangeWithScores(lb.ctx, entityKey, 0, int64(lb.config.K-1))
		_, err = pipe.Exec(lb.ctx)
		if err != nil && err != redis.Nil {
			return LeaderboardData{}, fmt.Errorf("failed to fetch entity data: %w", err)
		}

		if entityRankCmd.Err() == redis.Nil {
			data.EntityRank = -1
		} else if entityRankCmd.Err() != nil {
			return LeaderboardData{}, fmt.Errorf("failed to get entity rank: %w", entityRankCmd.Err())
		} else {
			data.EntityRank = int(entityRankCmd.Val())
		}

		if topKEntityCmd.Err() != nil {
			return LeaderboardData{}, fmt.Errorf("failed to fetch top-k entity: %w", topKEntityCmd.Err())
		}
		for _, m := range topKEntityCmd.Val() {
			data.TopKEntity = append(data.TopKEntity, User{
				ID:     m.Member.(string),
				Entity: data.Entity,
				Score:  m.Score,
			})
		}
	} else {
		data.EntityRank = -1
	}

	return data, nil
}

// GetTopKGlobal returns top k users across all entities.
// Ordered by score descending.
// Includes entity information for each user.
// Returns error if no users exist or Redis fails.
func (lb *Leaderboard) GetTopKGlobal() ([]User, error) {
	globalKey := lb.config.Namespace + ":global"
	entitiesKey := lb.config.Namespace + ":user:entities"

	members, err := lb.client.ZRevRangeWithScores(lb.ctx, globalKey, 0, int64(lb.config.K-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch global top-k: %w", err)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("no users in global leaderboard")
	}

	pipe := lb.client.Pipeline()
	entityCmds := make(map[string]*redis.StringCmd)
	for _, m := range members {
		userID := m.Member.(string)
		entityCmds[userID] = pipe.HGet(lb.ctx, entitiesKey, userID)
	}
	_, err = pipe.Exec(lb.ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to fetch entities: %w", err)
	}
	users := make([]User, 0, len(members))
	for _, m := range members {
		userID := m.Member.(string)
		users = append(users, User{
			ID:     userID,
			Entity: entityCmds[userID].Val(),
			Score:  m.Score,
		})
	}
	return users, nil
}

// GetTopKEntity returns top k users in specific entity.
// Ordered by score descending.
// Returns error if:
// - no users in entity
// - Redis operation fails
func (lb *Leaderboard) GetTopKEntity(entity string) ([]User, error) {
	entityKey := lb.config.Namespace + ":entity:" + entity

	members, err := lb.client.ZRevRangeWithScores(lb.ctx, entityKey, 0, int64(lb.config.K-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch entity %s top-k: %w", entity, err)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("no users in entity %s", entity)
	}

	users := make([]User, 0, len(members))
	for _, m := range members {
		userID := m.Member.(string)
		users = append(users, User{
			ID:     userID,
			Entity: entity,
			Score:  m.Score,
		})
	}
	return users, nil
}

// GetRankGlobal returns user's position in global ranking.
// 0-based ranking (0 is highest score).
// Returns -1 if user not found.
func (lb *Leaderboard) GetRankGlobal(userID string) (int, error) {
	globalKey := lb.config.Namespace + ":global"

	rank, err := lb.client.ZRevRank(lb.ctx, globalKey, userID).Result()
	if err == redis.Nil {
		return -1, nil
	}
	if err != nil {
		return -1, fmt.Errorf("failed to get global rank: %w", err)
	}
	return int(rank), nil
}

// GetRankEntity returns user's position in entity ranking.
// 0-based ranking (0 is highest score).
// Returns -1 if:
// - user not found
// - user has no entity
// - user not in entity ranking
func (lb *Leaderboard) GetRankEntity(userID string) (int, error) {
	entitiesKey := lb.config.Namespace + ":user:entities"

	entity, err := lb.client.HGet(lb.ctx, entitiesKey, userID).Result()
	if err == redis.Nil {
		return -1, nil
	}
	if err != nil {
		return -1, fmt.Errorf("failed to get user entity: %w", err)
	}
	if entity == "" {
		return -1, nil
	}

	entityKey := lb.config.Namespace + ":entity:" + entity
	rank, err := lb.client.ZRevRank(lb.ctx, entityKey, userID).Result()
	if err == redis.Nil {
		return -1, nil
	}
	if err != nil {
		return -1, fmt.Errorf("failed to get entity rank: %w", err)
	}
	return int(rank), nil
}

// GetUserScore returns user's current score.
// Returns error if:
// - user not found
// - Redis operation fails
func (lb *Leaderboard) GetUserScore(userID string) (float64, error) {
	globalKey := lb.config.Namespace + ":global"
	score, err := lb.client.ZScore(lb.ctx, globalKey, userID).Result()
	if err == redis.Nil {
		return 0, fmt.Errorf("user %s not found", userID)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get score: %w", err)
	}
	return score, nil
}

// GetUserEntity returns user's entity identifier.
// Returns empty string if:
// - user not found
// - user has no entity
// Returns error if Redis operation fails.
func (lb *Leaderboard) GetUserEntity(userID string) (string, error) {
	entitiesKey := lb.config.Namespace + ":user:entities"
	entity, err := lb.client.HGet(lb.ctx, entitiesKey, userID).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user entity: %w", err)
	}
	return entity, nil
}


