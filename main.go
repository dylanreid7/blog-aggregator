package main

import (
	"blog-aggregator/internal/database"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	DB *database.Queries
}

func main() {

	godotenv.Load()
	port := os.Getenv("PORT")
	dbURL := os.Getenv("DB")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	apiCfg := apiConfig{
		DB: dbQueries,
	}

	mux := http.NewServeMux()
	corsMux := middlewareCors(mux)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: corsMux,
	}

	mux.HandleFunc("GET /v1/readiness", handlerReadiness)
	mux.HandleFunc("GET /v1/error", handlerError)
	mux.HandleFunc("GET /v1/users", apiCfg.handleGetUsers)
	mux.HandleFunc("POST /v1/users", apiCfg.handlerUsersPost)

	log.Printf("Serving on port: %s\n", port)
	log.Fatal(srv.ListenAndServe())
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Add("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(status)
	w.Write(dat)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func handlerError(w http.ResponseWriter, r *http.Request) {
	respondWithError(w, 500, "Internal server error")
}

func (cfg *apiConfig) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	apiKey, err := getApiKey(r.Header)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error with api key: ", err))
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	user, err := cfg.DB.GetUserByAPIKey(r.Context(), apiKey)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error getting api key from db: ", err))
	}
	respondWithJSON(w, http.StatusOK, databaseUserToUser(user))
}

func getApiKey(h http.Header) (string, error) {
	auth, ok := h["Authorization"]
	if !ok || len(auth) < 1 {
		return "", errors.New("no authorization")
	}
	return auth[0], nil
}

func (cfg *apiConfig) handlerUsersPost(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Name string
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error parsing JSON: ", err))
		return
	}
	id := uuid.New()
	currTime := time.Now().UTC()
	name := params.Name

	user, err := cfg.DB.CreateUser(r.Context(), database.CreateUserParams{
		ID:        id,
		CreatedAt: currTime,
		UpdatedAt: currTime,
		Name:      name,
	})

	if err != nil {
		respondWithError(w, 400, fmt.Sprintf("Couldn't create user: ", err))
		return
	}
	respondWithJSON(w, http.StatusOK, databaseUserToUser(user))
}
