package bpdecode

import "encoding/json"

type jsonModule struct {
	Entries []Entry `json:"entries"`
}

func RenderJSON(entries []Entry) ([]byte, error) {
	return json.Marshal(jsonModule{Entries: entries})
}
