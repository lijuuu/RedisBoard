package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	redisboard "github.com/lijuuu/RedisBoard" 
)

type Server struct {
	lb *redisboard.Leaderboard
}

func NewServer() (*Server, error) {
	cfg := redisboard.Config{
		Namespace:   "game1",
		K:           10,
		MaxUsers:    1_000_000,
		MaxEntities: 200,
		FloatScores: true,
		RedisAddr:   "localhost:6379",
	}
	lb, err := redisboard.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Server{lb: lb}, nil
}

func (s *Server) AddUser(w http.ResponseWriter, r *http.Request) {
	var user redisboard.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	if user.ID == "" || user.Score < 0 || len(user.Entity) > 2 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}
	if err := s.lb.AddUser(user); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("User %s added", user.ID)})
}

func (s *Server) RemoveUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user ID"})
		return
	}
	if err := s.lb.RemoveUser(userID); err != nil {
		if err.Error() == "invalid user ID" {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("User %s removed", userID)})
}

func (s *Server) IncrementScore(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user ID"})
		return
	}
	entity := r.URL.Query().Get("entity")
	scoreStr := r.URL.Query().Get("score")
	score, err := strconv.ParseFloat(scoreStr, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid score"})
		return
	}
	if err := s.lb.IncrementScore(userID, entity, score); err != nil {
		if err.Error() == "invalid user ID or score increment" {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("Score incremented for user %s", userID)})
}

func (s *Server) DecrementScore(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user ID"})
		return
	}
	entity := r.URL.Query().Get("entity")
	scoreStr := r.URL.Query().Get("score")
	score, err := strconv.ParseFloat(scoreStr, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid score"})
		return
	}
	if err := s.lb.IncrementScore(userID, entity, -score); err != nil {
		if err.Error() == "invalid user ID or score increment" {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("Score decremented for user %s", userID)})
}

func (s *Server) GetTopKGlobal(w http.ResponseWriter, r *http.Request) {
	users, err := s.lb.GetTopKGlobal()
	if err != nil {
		if err.Error() == "no users in global leaderboard" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(users)
}

func (s *Server) GetTopKEntity(w http.ResponseWriter, r *http.Request) {
	entity := mux.Vars(r)["entity"]
	if entity == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid entity"})
		return
	}
	users, err := s.lb.GetTopKEntity(entity)
	if err != nil {
		if strings.Contains(err.Error(), "no users in entity") {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(users)
}

func (s *Server) GetUserRank(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user ID"})
		return
	}
	globalRank, err := s.lb.GetRankGlobal(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	entityRank, err := s.lb.GetRankEntity(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	response := map[string]int{
		"globalRank": globalRank,
		"entityRank": entityRank,
	}
	json.NewEncoder(w).Encode(response)
}

func (s *Server) GetLeaderboardData(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user ID"})
		return
	}
	data, err := s.lb.GetUserLeaderboardData(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(data)
}

func (s *Server) UpdateEntityByUserID(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	newEntity := mux.Vars(r)["entityID"] // Fixed from "entity"
	if userID == "" || newEntity == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user ID or entity"})
		return
	}
	currentEntity, _ := s.lb.GetUserEntity(userID)
	if currentEntity == newEntity {
		json.NewEncoder(w).Encode(map[string]string{"message": "Entity unchanged"})
		return
	}
	err := s.lb.UpdateEntityByUserID(userID, newEntity)
	if err != nil {
		if err.Error() == "invalid user ID" || err.Error() == "invalid new entity" || strings.Contains(err.Error(), "not found") {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("Entity updated to %s for user %s", newEntity, userID)})
}


func main() {
	srv, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer srv.lb.Close()

	// Generate 1 million mock users
	countries := []string{"US", "UK", "CA", "DE", "FR"}
	start := time.Now()
	log.Println("Generating 1 million mock users...")
	for i := 0; i < 1_000_000; i++ {
		userID := fmt.Sprintf("user%d", i)
		score := rand.Float64() * 1000
		entity := countries[rand.IntN(len(countries))]
		err := srv.lb.AddUser(redisboard.User{
			ID:     userID,
			Entity: entity,
			Score:  score,
		})
		if err != nil {
			log.Fatalf("Failed to add test user %s: %v", userID, err)
		}
		if i%100_000 == 0 && i > 0 {
			log.Printf("Added %d users...", i)
		}
	}
	duration := time.Since(start)
	log.Printf("Added 1 million users in %v", duration)

	r := mux.NewRouter()
	r.HandleFunc("/user", srv.AddUser).Methods("POST")
	r.HandleFunc("/user/{userID}", srv.RemoveUser).Methods("DELETE")
	r.HandleFunc("/user/{userID}/increment", srv.IncrementScore).Methods("POST")
	r.HandleFunc("/user/{userID}/decrement", srv.DecrementScore).Methods("POST")
	r.HandleFunc("/topk/global", srv.GetTopKGlobal).Methods("GET")
	r.HandleFunc("/topk/entity/{entity}", srv.GetTopKEntity).Methods("GET")
	r.HandleFunc("/rank/{userID}", srv.GetUserRank).Methods("GET")
	r.HandleFunc("/leaderboard/{userID}", srv.GetLeaderboardData).Methods("GET")
	r.HandleFunc("/user/{userID}/{entityID}", srv.UpdateEntityByUserID).Methods("PUT")

	log.Println("Server starting on :3000")
	log.Fatal(http.ListenAndServe(":3000", r))
}