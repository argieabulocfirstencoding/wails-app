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
	// https://firstencoding.app/version-test/api/1.1/wf/employee_details
	user, err := u.GetUserFromId(id)
	if err != nil {
		return err
	}

	// FIX development mode only
	url := "employee_details"
	method := "POST"

	email := user.Email
	if user.Email == "test@fes.com" {
		email = "johnpaultunacao.firstencoding@gmail.com"
	}

	body := map[string]any{
		"email": email,
	}

	request := types.Request{
		Url:    url,
		Method: method,
		Body:   body,
	}

	response, err := u.fesClient.Request(request)
	if err != nil {
		return err
	}

	employeeRaw, exists := response["employee"]
	if !exists || employeeRaw == nil {
		return nil
	}

	switch v := employeeRaw.(type) {
	case map[string]any:
		employee := v

		fesid, _ := response["fesid"].(string)
		rawBirthdate, _ := employee["BirthDate txt"].(string)

		birthdate := ""
		if rawBirthdate != "" {
			slicedBd := strings.Split(rawBirthdate, " ")

			if len(slicedBd) >= 3 {
				birthdate = fmt.Sprintf("%s %s %s", slicedBd[0], slicedBd[1], slicedBd[2])
			}

		}
		u.logger.Println("Birthdate:", rawBirthdate)
		u.logger.Println("fesid:", fesid)

		if fesid != "" {
			user.Name = employee["Full Name"].(string)
			user.FirstName = employee["FirstName"].(string)
			user.LastName = employee["LastName"].(string)
			user.MiddleName = employee["MiddleName"].(string)
			user.FESID = response["fesid"].(string)
			user.Photo = response["photo"].(string)
			user.Position = response["position"].(string)
			user.Phone = employee["Account phone number"].(string)
			user.Address = employee["Address"].(string)
			user.Birthdate = birthdate
			user.SupervisorId = response["immediate_supervisor"].(string)
			user.EmployeeId = response["employee_unique_id"].(string)
			user.EmploymentId = response["employeement_unique_id"].(string)

			if err := db.DB.Save(&user).Error; err != nil {
				u.logger.Println("Failed to save user:", err)
			}

			runtime.EventsEmit(ctx, "employee_details_fetched")
		}

		return nil
		// use employee
	case []any:
		// employee is a list
		return nil
	default:
		// unexpected type
		return nil
	}

}
