package redisboard

import (
	"testing"
)

func newTestLeaderboard(t *testing.T, cfg Config) *Leaderboard {
	t.Helper()
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}
	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("create leaderboard: %v", err)
	}
	return lb
}

func TestNew(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	if lb.config.Namespace != "test" {
		t.Errorf("expected namespace test, got %s", lb.config.Namespace)
	}
	if lb.config.K != 10 {
		t.Errorf("expected K 10, got %d", lb.config.K)
	}
}

func TestAddUser(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	user := User{ID: "u1", Entity: "US", Score: 100}
	if err := lb.AddUser(user); err != nil {
		t.Errorf("AddUser: %v", err)
	}

	score, err := lb.GetUserScore("u1")
	if err != nil || score != 100 {
		t.Errorf("expected score 100, got %f, err: %v", score, err)
	}
}

func TestIncrementScore(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	if err := lb.IncrementScore("u1", "US", 50); err != nil {
		t.Errorf("IncrementScore: %v", err)
	}

	score, err := lb.GetUserScore("u1")
	if err != nil || score != 150 {
		t.Errorf("expected score 150, got %f, err: %v", score, err)
	}
}

func TestRemoveUser(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	if err := lb.RemoveUser("u1"); err != nil {
		t.Errorf("RemoveUser: %v", err)
	}

	_, err := lb.GetUserScore("u1")
	if err == nil {
		t.Error("expected user not found error")
	}
}

func TestUpdateEntityByUserID(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	if err := lb.UpdateEntityByUserID("u1", "UK"); err != nil {
		t.Errorf("UpdateEntityByUserID: %v", err)
	}

	entity, err := lb.GetUserEntity("u1")
	if err != nil || entity != "UK" {
		t.Errorf("expected entity UK, got %s, err: %v", entity, err)
	}
}

func TestGetUserLeaderboardData(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test", K: 2})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	lb.AddUser(User{ID: "u2", Entity: "US", Score: 80})

	data, err := lb.GetUserLeaderboardData("u1")
	if err != nil {
		t.Errorf("GetUserLeaderboardData: %v", err)
	}
	if data.UserID != "u1" || data.Score != 100 || data.Entity != "US" || data.GlobalRank != 0 || data.EntityRank != 0 {
		t.Errorf("unexpected data: %+v", data)
	}
}

func TestGetTopKGlobal(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test", K: 2})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	lb.AddUser(User{ID: "u2", Entity: "UK", Score: 80})

	topK, err := lb.GetTopKGlobal()
	if err != nil {
		t.Errorf("GetTopKGlobal: %v", err)
	}
	if len(topK) != 2 || topK[0].ID != "u1" || topK[0].Score != 100 {
		t.Errorf("unexpected topK: %+v", topK)
	}
}

func TestGetTopKEntity(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test", K: 2})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	lb.AddUser(User{ID: "u2", Entity: "US", Score: 80})

	topK, err := lb.GetTopKEntity("US")
	if err != nil {
		t.Errorf("GetTopKEntity: %v", err)
	}
	if len(topK) != 2 || topK[0].ID != "u1" || topK[0].Score != 100 {
		t.Errorf("unexpected topK: %+v", topK)
	}
}

func TestGetRankGlobal(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	rank, err := lb.GetRankGlobal("u1")
	if err != nil || rank != 0 {
		t.Errorf("expected rank 0, got %d, err: %v", rank, err)
	}
}

func TestGetRankEntity(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	rank, err := lb.GetRankEntity("u1")
	if err != nil || rank != 0 {
		t.Errorf("expected rank 0, got %d, err: %v", rank, err)
	}
}

func TestGetUserScore(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	score, err := lb.GetUserScore("u1")
	if err != nil || score != 100 {
		t.Errorf("expected score 100, got %f, err: %v", score, err)
	}
}

func TestGetUserEntity(t *testing.T) {
	lb := newTestLeaderboard(t, Config{Namespace: "test"})
	defer lb.Close()

	lb.AddUser(User{ID: "u1", Entity: "US", Score: 100})
	entity, err := lb.GetUserEntity("u1")
	if err != nil || entity != "US" {
		t.Errorf("expected entity US, got %s, err: %v", entity, err)
	}
}