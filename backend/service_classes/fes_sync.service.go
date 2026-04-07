package serviceclasses

import (
	"context"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"sync"
	"time"
)

type FesSyncService struct {
	ctx         context.Context
	isConnected bool
	AuthUser    *models.FirebaseLoginResponse
	IdToken     *string
	IssuedAt    *time.Time
	Backblaze   *services.BackblazeSync

	connecting bool
	queue      []func()
	mu         sync.Mutex
}

func NewFesSyncService() *FesSyncService {
	backblaze := services.NewBackblazeSync()
	return &FesSyncService{
		Backblaze: backblaze,
	}
}

func (s *FesSyncService) Connect(ctx context.Context) error {
	s.ctx = ctx

	return nil
}

func (s *FesSyncService) SyncCredentials() {

}

func (s *FesSyncService) SyncTasks() {

}

func (s *FesSyncService) SyncTaskRecord(taskRecord *models.TaskRecord) error {
	return nil
}

func (s *FesSyncService) SyncAttendance(attendance *models.Attendance) error {
	return nil
}

func (s *FesSyncService) SyncAllAttendance() error {
	return nil
}

func (s *FesSyncService) SyncUnsyncedRecords() error {
	return nil
}

func (s *FesSyncService) SyncUser(user models.AuthUser) error {
	return nil
}

func (s *FesSyncService) TestSync() bool {
	return true
}

func (s *FesSyncService) RefreshAccessToken() {

}
