package service_classes

import (
	"context"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"sync"
	"time"
)

type SyncService struct {
	ctx         context.Context
	isConnected bool
	AuthUser    *models.FirebaseLoginResponse
	IdToken     *string
	IssuedAt    *time.Time
	Backblaze   *services.BackblazeSync
	fesClient   *FesClientService

	connecting bool
	queue      []func()
	mu         sync.Mutex
}

func NewSyncService() *SyncService {
	backblaze := services.NewBackblazeSync()
	return &SyncService{
		Backblaze: backblaze,
		fesClient: NewFesClientService(),
	}
}

func (s *SyncService) Connect(ctx context.Context) error {
	s.ctx = ctx

	return nil
}

func (s *SyncService) SyncCredentials() {

}

func (s *SyncService) SyncTasks() {

}

func (s *SyncService) SyncTaskRecord(taskRecord *models.TaskRecord) error {
	return nil
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) error {
	return nil
}

func (s *SyncService) SyncAllAttendance() error {
	return nil
}

func (s *SyncService) SyncUnsyncedRecords() error {
	return nil
}

func (s *SyncService) SyncUser(user models.AuthUser) error {
	return nil
}

func (s *SyncService) TestSync() bool {
	return true
}

func (s *SyncService) RefreshAccessToken() {

}
