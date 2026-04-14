package service_classes

import (
	"context"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type UserService struct {
	ctx       context.Context
	fesClient *FesClientService
	userRepo  *repository.UserRepository
	logger    *utils.LoggerService
}

func NewUserService() *UserService {
	return &UserService{
		userRepo:  repository.NewUserRepository(),
		fesClient: NewFesClientService(),
		logger:    utils.NewLoggerService(),
	}
}

func (u *UserService) GetUserFromId(id string) (*models.User, error) {
	return u.userRepo.GetUserFromId(id)
}

func (u *UserService) CreateUser(id string, email string) (*models.User, error) {
	return u.userRepo.CreateUser(id, email)
}

func (u *UserService) FetchEmployeeDetails(ctx context.Context, id string) error {
	user, err := u.GetUserFromId(id)
	if err != nil {
		return err
	}

	url := "wf/employee_details"
	method := "POST"

	email := user.Email
	if user.Email == "test@fes.com" {
		email = "johnpaultunacao.firstencoding@gmail.com"
	}

	request := types.Request{
		Url:    url,
		Method: method,
		Body:   map[string]any{"email": email},
	}

	response, err := u.fesClient.Request(request)
	if err != nil {
		return err
	}

	employeeRaw, exists := response["employee"]
	if !exists || employeeRaw == nil {
		u.logger.Println("Employee not found")
		return nil
	}

	// --- Helper function to prevent nil-to-string conversion crashes ---
	safeString := func(m map[string]any, key string) string {
		val, exists := m[key]
		if !exists || val == nil {
			return ""
		}
		s, ok := val.(string)
		if !ok {
			return ""
		}
		return s
	}

	var employee map[string]any

	// Handle both Map and Slice responses
	switch v := employeeRaw.(type) {
	case map[string]any:
		employee = v
	case []any:
		if len(v) > 0 {
			employee, _ = v[0].(map[string]any)
		}
	}

	if employee == nil {
		u.logger.Println("Could not resolve employee map")
		return nil
	}

	// 1. Extract and format birthdate safely
	rawBirthdate := safeString(employee, "BirthDate txt")
	birthdate := ""
	if rawBirthdate != "" {
		slicedBd := strings.Split(rawBirthdate, " ")
		if len(slicedBd) >= 3 {
			birthdate = fmt.Sprintf("%s %s %s", slicedBd[0], slicedBd[1], slicedBd[2])
		}
	}

	// 2. Map fields safely (This prevents the "interface {} is nil" error)
	fesid := safeString(response, "fesid")
	if fesid != "" {
		user.Name = safeString(employee, "Full Name")
		user.FirstName = safeString(employee, "FirstName")
		user.LastName = safeString(employee, "LastName")
		user.MiddleName = safeString(employee, "MiddleName")

		user.FESID = fesid
		user.Photo = safeString(response, "photo")
		user.Position = safeString(response, "position")
		user.Phone = safeString(employee, "Account phone number")
		user.Address = safeString(employee, "Address")
		user.Birthdate = birthdate
		user.SupervisorId = safeString(response, "immediate_supervisor")
		user.EmployeeId = safeString(response, "employee_unique_id")
		user.EmploymentId = safeString(response, "employeement_unique_id")

		if err := db.DB.Save(&user).Error; err != nil {
			u.logger.Println("Failed to save user:", err)
			return err
		}

		runtime.EventsEmit(ctx, "employee_details_fetched")
		u.logger.Println("Saved employee details successfully for:", user.Name)
	}

	return nil
}
