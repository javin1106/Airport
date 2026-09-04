package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	httpapi "github.com/javin1106/airport/internal/httpapi"
	"github.com/javin1106/airport/internal/storage"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := healthResponse{
		Status:  "ok",
		Service: "airport-api",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	useSSL, err := strconv.ParseBool(os.Getenv("S3_USE_SSL"))
	if err != nil {
		log.Fatalf("invalid S3_USE_SSL value: %v", err)
	}

	objectStore, err := storage.New(storage.Config{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket:    os.Getenv("S3_BUCKET"),
		UseSSL:    useSSL,
	})
	if err != nil {
		log.Fatalf("configure object storage: %v", err)
	}

	startupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := objectStore.EnsureBucket(startupContext); err != nil {
		log.Fatalf("prepare object storage: %v", err)
	}

	deployHandler := httpapi.NewDeployHandler(objectStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /deploy", deployHandler.Handle)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Airport API is listening on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
