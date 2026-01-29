package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
)

func HealthHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sqlDB, _ := s.DB.DB()
		err := sqlDB.Ping()
		ok := err == nil
		resp := models.APIResponse{
			Success: ok,
			Message: "ok",
			Data: map[string]interface{}{
				"db":   ok,
				"time": time.Now(),
			},
		}
		if !ok {
			resp.Success = false
			resp.Message = "db unreachable"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
