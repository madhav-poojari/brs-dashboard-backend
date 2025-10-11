package utils

import (
	"encoding/json"
	"net/http"

	"github.com/madhava-poojari/dashboard-api/internal/models" // Adjust the import path as necessary
)

func WriteJSONResponse(w http.ResponseWriter, statusCode int, success bool, message string, data interface{}, errDetail interface{}) {
	response := models.APIResponse{
		Success: success,
		Message: message,
		Data:    data,
		Error:   errDetail,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
