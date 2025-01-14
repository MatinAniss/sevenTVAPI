package events

import (
	"encoding/json"

	"github.com/seventv/common/structures/v3"
)

type BridgedCommandBody interface {
	json.RawMessage | UserStateCommandBody
}

type UserStateCommandBody struct {
	Platform    structures.UserConnectionPlatform `json:"platform"`
	Identifiers []string                          `json:"identifiers"`
	Kinds       []structures.CosmeticKind         `json:"kinds"`
}
