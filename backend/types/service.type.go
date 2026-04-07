package types

import (
	"context"
	"fesapp/backend/db/models"
	"sync"

	"github.com/kurin/blazer/b2"
)

type BackblazeSync struct {
	ctx         context.Context
	isConnected bool
	connecting  bool
	AuthUser    *models.FirebaseLoginResponse
	Bucket      *b2.Bucket

	queue []func()
	mu    sync.Mutex
}
