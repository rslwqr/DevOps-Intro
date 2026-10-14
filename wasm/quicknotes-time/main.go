package main

import (
	"encoding/json"
	"net/http"
	"time"
	_ "time/tzdata"

	spinhttp "github.com/spinframework/spin-go-sdk/v3/http"
)

type TimeResponse struct {
	Unix       int64  `json:"unix"`
	ISO        string `json:"iso"`
	HourMinute string `json:"hour_minute"`
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		location, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			http.Error(w, "Failed to load timezone", http.StatusInternalServerError)
			return
		}

		now := time.Now().In(location)

		response := TimeResponse{
			Unix:       now.Unix(),
			ISO:        now.Format(time.RFC3339),
			HourMinute: now.Format("15:04"),
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})
}

// Required by the compiler but not executed by Spin.
func main() {}
