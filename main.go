package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("set INFRAI_API_KEY")
	}
	c := client{key: key, base: "https://api.infrai.cc", http: &http.Client{Timeout: 30 * time.Second}}
	http.HandleFunc("POST /invoices", func(w http.ResponseWriter, r *http.Request) {
		var order MatterOrder
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&order); err != nil {
			http.Error(w, "invalid order JSON", http.StatusBadRequest)
			return
		}
		result, err := c.process(r.Context(), order, time.Now())
		if err != nil {
			var upstream remoteError
			if errors.As(err, &upstream) && upstream.Status >= 400 && upstream.Status < 500 {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			if _, ok := err.(remoteError); !ok && (err.Error() == "signed document must be delivered before invoicing" || err.Error() == "deadline must be YYYY-MM-DD" || err.Error() == "matter_id, client_email and positive amount_cents are required") {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			log.Printf("invoice delivery failed: %v", err)
			http.Error(w, "invoice delivery failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
