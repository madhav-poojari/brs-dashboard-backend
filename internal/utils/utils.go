package utils

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func GenerateID() string {
	return uuid.NewString()
}

func StringifyJSON(j datatypes.JSON) string {
	var arr []string
	_ = json.Unmarshal([]byte(j), &arr)
	// join into bullet points
	out := ""
	for i, s := range arr {
		out += "- " + s
		if i+1 < len(arr) {
			out += "\n"
		}
	}
	return out
}

func DatatypesJSONFromStrings(ss []string) datatypes.JSON {
	b, _ := json.Marshal(ss)
	return datatypes.JSON(b)
}

func DatatypesJSONFromMap(m map[string]interface{}) datatypes.JSON {
	if m == nil {
		m = map[string]interface{}{}
	}
	b, _ := json.Marshal(m)
	return datatypes.JSON(b)
}
