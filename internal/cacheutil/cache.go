package cacheutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	internaltypes "github.com/duxweb/oro/internal/types"
)

type Payload struct {
	Rows []internaltypes.Map `json:"rows"`
}

func EncodeRows(rows []internaltypes.Map) ([]byte, error) {
	return json.Marshal(Payload{Rows: rows})
}

func DecodeRows(value []byte) ([]internaltypes.Map, error) {
	var payload Payload
	if err := json.Unmarshal(value, &payload); err != nil {
		return nil, err
	}
	if payload.Rows == nil {
		return []internaltypes.Map{}, nil
	}
	return payload.Rows, nil
}

func Key(values internaltypes.Map) (string, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "oro:" + hex.EncodeToString(sum[:]), nil
}
