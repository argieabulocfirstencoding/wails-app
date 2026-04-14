package service_classes

import (
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
)

type AttendanceService struct {
	attendanceRepo *repository.AttendanceRepository
}

func NewAttendanceService() *AttendanceService {
	return &AttendanceService{
		attendanceRepo: &repository.AttendanceRepository{},
	}
}

func (a *AttendanceService) LoginAttendance(userId string) (*models.Attendance, error) {
	return a.attendanceRepo.CreateAttendance(userId)
}

func (a *AttendanceService) LogoutAttendance(attendance *models.Attendance) error {
	return a.attendanceRepo.LogoutAttendance(attendance)
}
