package service_classes

import (
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"time"
)

type AttendanceService struct {
	AttendanceRepo *repository.AttendanceRepository
	SyncService    *SyncService
}

func NewAttendanceService(SyncService *SyncService) *AttendanceService {
	return &AttendanceService{
		AttendanceRepo: repository.NewAttendanceRepository(),
		SyncService:    SyncService,
	}
}

func (a *AttendanceService) LoginAttendance(userId string) (*models.Attendance, error) {
	return a.AttendanceRepo.CreateAttendance(userId)

}

func (a *AttendanceService) LogoutAttendance(attendance *models.Attendance) error {
	return a.AttendanceRepo.LogoutAttendance(attendance)
}

func (a *AttendanceService) LogoutAttendanceAt(attendance *models.Attendance, endedAt time.Time) error {
	return a.AttendanceRepo.LogoutAttendanceAt(attendance, endedAt)
}

func (a *AttendanceService) GetAttendanceFromID(id uint) (*models.Attendance, error) {
	return a.AttendanceRepo.GetAttendanceFromID(id)
}

func (a *AttendanceService) GetOpenAttendanceFromUserID(userID string) (*models.Attendance, error) {
	return a.AttendanceRepo.GetOpenAttendanceFromUserID(userID)
}
