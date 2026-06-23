package utils

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func NewCorrelationID(scope string) string {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	id = id[:12]
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "op"
	}
	return fmt.Sprintf("%s-%s", scope, id)
}
