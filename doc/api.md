# RedisBoard API Reference

This document details the **RedisBoard** Go library’s functions, parameters, and usage for building leaderboards with global and entity-based rankings. It assumes you’ve read the [README](../README.md) for setup and context.

## Overview

**RedisBoard** provides a clean API to manage leaderboards using Redis sorted sets and hashes. It supports:
- Adding/removing users.
- Updating scores or entities (e.g., moving a user from US to UK).
- Fetching global/entity ranks, top-k lists, and user data.
- Atomic operations via Redis pipelining for consistency.

All functions operate on a `Leaderboard` instance, created with a `Config` struct defining settings like namespace and Redis address. Operations are fast (~100µs for rank fetches) and scale to millions of users.

## Configuration

Start by creating a `Leaderboard` with a `Config` struct, which accepts:
- **Namespace**: String prefix for Redis keys (e.g., `game1`). Default: `default`.
- **K**: Number of top users to track (e.g., 10). Default: 10.
- **MaxUsers**: Max allowed users (e.g., 1,000,000). Default: 1M. (Note: Currently unenforced.)
- **MaxEntities**: Max entity groups (e.g., 200). Default: 200. (Note: Currently unenforced.)
- **FloatScores**: True for decimal scores, false for integers. Default: false.
- **RedisAddr**: Redis server address (e.g., `localhost:6379`). Default: `localhost:6379`.
- **RedisPass**: Optional Redis password. Default: empty.

## Data Structures

- **User**:
  - **ID**: String, unique user identifier (e.g., `player42`).
  - **Entity**: String, optional group like a country code (e.g., `US`). Can be empty.
  - **Score**: Float64, user’s score (e.g., 550.5). Non-negative.

- **LeaderboardData**:
  - **UserID**: String, user’s ID.
  - **Score**: Float64, current score.
  - **Entity**: String, user’s entity (or empty).
  - **GlobalRank**: Int, 0-based global rank (0 = top). -1 if not ranked.
  - **EntityRank**: Int, 0-based entity rank. -1 if no entity or not ranked.
  - **TopKGlobal**: Slice of `User`, top-k users globally.
  - **TopKEntity**: Slice of `User`, top-k in user’s entity (empty if no entity).

## Functions

Below are **RedisBoard**’s public functions, their purposes, parameters, and return values.

1. **New**
   - **Purpose**: Creates a new `Leaderboard` instance connected to Redis.
   - **Parameters**:
     - `cfg`: `Config` struct (namespace, Redis address, etc.).
   - **Returns**:
     - `*Leaderboard`: Leaderboard instance.
     - `error`: If Redis connection fails.
   - **Notes**: Call `Close` when done to free resources.

2. **Close**
   - **Purpose**: Shuts down the Redis connection.
   - **Parameters**: None.
   - **Returns**:
     - `error`: If closing fails (rare).
   - **Notes**: Ensures proper cleanup.

3. **AddUser**
   - **Purpose**: Adds or updates a user’s score and entity in global/entity rankings.
   - **Parameters**:
     - `user`: `User` struct (ID, entity, score).
   - **Returns**:
     - `error`: If ID is empty, score is negative, or Redis fails.
   - **Notes**: Atomic via pipelining. Entity can be empty (no entity ranking).

4. **IncrementScore**
   - **Purpose**: Adds (or subtracts) a value to a user’s score, optionally updating their entity.
   - **Parameters**:
     - `userID`: String, user’s ID.
     - `entity`: String, new entity (or empty to keep current).
     - `scoreIncrement`: Float64, amount to add (negative to subtract).
   - **Returns**:
     - `error`: If ID is empty, increment is zero, or Redis fails.
   - **Notes**: Updates global and entity rankings atomically.

5. **DecrementScore**
   - **Purpose**: Subtracts a value from a user's score, optionally updating their entity.
   - **Parameters**:
     - `userID`: String, user's ID.
     - `entity`: String, new entity (or empty to keep current).
     - `scoreDecrement`: Float64, amount to subtract.
   - **Returns**:
     - `error`: If ID is empty, decrement is zero, or Redis fails.
   - **Notes**: Updates global and entity rankings atomically.

6. **RemoveUser**
   - **Purpose**: Deletes a user from all rankings and entity mappings.
   - **Parameters**:
     - `userID`: String, user’s ID.
   - **Returns**:
     - `error`: If ID is empty or Redis fails.
   - **Notes**: Safe if user doesn’t exist.

7. **UpdateEntityByUserID**
   - **Purpose**: Moves a user to a new entity, preserving their score.
   - **Parameters**:
     - `userID`: String, user’s ID.
     - `newEntity`: String, target entity (e.g., `UK`).
   - **Returns**:
     - `error`: If ID is empty, user doesn’t exist, new entity is empty, or Redis fails.
   - **Notes**: Removes from old entity’s ranking, adds to new one. Atomic.

8. **GetUserLeaderboardData**
   - **Purpose**: Fetches a user’s full leaderboard info (score, ranks, top-k lists).
   - **Parameters**:
     - `userID`: String, user’s ID.
   - **Returns**:
     - `LeaderboardData`: Struct with user’s data and top-k lists.
     - `error`: If Redis fails.
   - **Notes**: Returns `-1` ranks and zero score for non-existent users.

9. **GetTopKGlobal**
   - **Purpose**: Gets the top k users across all entities.
   - **Parameters**: None.
   - **Returns**:
     - `[]User`: Slice of top users (ID, entity, score).
     - `error`: If no users exist or Redis fails.
   - **Notes**: Ordered by score descending.

10. **GetTopKEntity**
    - **Purpose**: Gets the top k users in a specific entity.
    - **Parameters**:
      - `entity`: String, entity code (e.g., `US`).
    - **Returns**:
      - `[]User`: Slice of top users in entity.
      - `error`: If entity is empty or Redis fails.
    - **Notes**: Errors if no users in entity.

11. **GetRankGlobal**
    - **Purpose**: Gets a user’s global rank (0-based).
    - **Parameters**:
      - `userID`: String, user’s ID.
    - **Returns**:
      - `int`: Rank (0 = top). -1 if not found.
      - `error`: If Redis fails.
    - **Notes**: Fast O(log n) lookup.

12. **GetRankEntity**
    - **Purpose**: Gets a user’s rank within their entity.
    - **Parameters**:
      - `userID`: String, user’s ID.
    - **Returns**:
      - `int`: Rank. -1 if no entity or not found.
      - `error`: If Redis fails.
    - **Notes**: Checks user’s entity first.

13. **GetUserScore**
    - **Purpose**: Gets a user’s current score.
    - **Parameters**:
      - `userID`: String, user’s ID.
    - **Returns**:
      - `float64`: Score. 0 if not found.
      - `error`: If user doesn’t exist or Redis fails.
    - **Notes**: Simple score lookup.

14. **GetUserEntity**
    - **Purpose**: Gets a user’s entity.
    - **Parameters**:
      - `userID`: String, user’s ID.
    - **Returns**:
      - `string`: Entity (or empty if none).
      - `error`: If Redis fails.
    - **Notes**: Returns empty string for non-existent users.
