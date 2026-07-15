package service_classes

import (
	"context"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"net/mail"
	"strings"
	"time"

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

func (u *UserService) FetchEmployeeDetails(ctx context.Context, user *models.User) error {

	if user == nil {
		return u.logger.Error("no user specified for fetching empoyee details")
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
	birthdate := extractBirthdate(employee)

	// if rawBirthdate != "" {
	// 	slicedBd := strings.Split(rawBirthdate, " ")

	// 	// Check if we have enough parts to avoid an index out of range error
	// 	if len(slicedBd) < 3 {
	// 		// Handle the "error" state here if needed
	// 		fmt.Println("Warning: rawBirthdate format is invalid")
	// 	} else {
	// 		birthdate = fmt.Sprintf("%s %s %s", slicedBd[0], slicedBd[1], slicedBd[2])
	// 	}
	// }

	// 2. Map fields safely (This prevents the "interface {} is nil" error)
	hydrateEmployeeEmail(user, response, employee)

	fesid := employeeDetailsString(response, "fesid")
	if fesid != "" {
		user.Name = employeeDetailsString(employee, "Full Name")
		user.FirstName = employeeDetailsString(employee, "FirstName")
		user.LastName = employeeDetailsString(employee, "LastName")
		user.MiddleName = employeeDetailsString(employee, "MiddleName")

		user.FESID = fesid
		user.Photo = employeeDetailsString(response, "photo")
		user.Position = employeeDetailsString(response, "position")
		user.Phone = employeeDetailsString(employee, "Account phone number")
		user.Address = employeeDetailsString(employee, "Address")
		user.Birthdate = birthdate
		user.SupervisorId = employeeDetailsString(response, "immediate_supervisor")
		user.EmployeeId = employeeDetailsString(response, "employee_unique_id")
		user.EmploymentId = employeeDetailsString(response, "employeement_unique_id")

		if err := db.DB.Save(&user).Error; err != nil {
			u.logger.Println("Failed to save user:", err)
			return err
		}

		runtime.EventsEmit(ctx, "employee_details_fetched")
		u.logger.Println("Saved employee details successfully for:", user.Name)
	}

	return nil
}

func employeeDetailsString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}

	value, exists := values[key]
	if !exists || value == nil {
		return ""
	}

	text, ok := value.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(text)
}

// extractEmployeeEmail returns the canonical email exposed by the employee
// details endpoint. The top-level response takes precedence because it is the
// workflow's authenticated-user value; employee-record fields support the
// existing Bubble data shapes.
func extractEmployeeEmail(response, employee map[string]any) string {
	if email := canonicalEmailFromFields(response); email != "" {
		return email
	}

	return canonicalEmailFromFields(employee)
}

func canonicalEmailFromFields(values map[string]any) string {
	if values == nil {
		return ""
	}

	normalizedValues := make(map[string]string, len(values))
	for key := range values {
		normalizedValues[normalizeEmailFieldName(key)] = employeeDetailsString(values, key)
	}

	for _, fieldName := range []string{
		"email",
		"useremail",
		"accountemail",
		"emailaddress",
		"accountemailaddress",
		"googleemail",
	} {
		if email := normalizedEmployeeEmail(normalizedValues[fieldName]); email != "" {
			return email
		}
	}

	return ""
}

func normalizeEmailFieldName(value string) string {
	var normalized strings.Builder
	for _, char := range strings.ToLower(value) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			normalized.WriteRune(char)
		}
	}

	return normalized.String()
}

func hydrateEmployeeEmail(user *models.User, response, employee map[string]any) {
	if user == nil {
		return
	}

	if email := extractEmployeeEmail(response, employee); email != "" {
		user.Email = email
		return
	}

	if normalizedEmployeeEmail(user.Email) == "" {
		// An FES ID is a valid login identifier but must never be retained as
		// the user's email or used to select a Chrome profile.
		user.Email = ""
	}
}

func normalizedEmployeeEmail(value string) string {
	email := strings.TrimSpace(value)
	if email == "" {
		return ""
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || !strings.Contains(address.Address, "@") {
		return ""
	}

	return address.Address
}

func resolvedAuthenticatedEmployeeEmail(user *models.User) string {
	if user == nil {
		return ""
	}

	return normalizedEmployeeEmail(user.Email)
}

func extractBirthdate(employee map[string]any) string {
	raw, exists := employee["BirthDate txt"]
	if !exists || raw == nil {
		raw = employee["Birthdate"]
	}
	if raw == nil {
		raw = employee["Birthday"]
	}

	switch v := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return ""
		}
		slicedBd := strings.Split(trimmed, " ")
		if len(slicedBd) >= 3 {
			return fmt.Sprintf("%s %s %s", slicedBd[0], slicedBd[1], slicedBd[2])
		}
		return trimmed
	case float64:
		// Bubble sometimes returns numeric timestamps in milliseconds.
		if v <= 0 {
			return ""
		}
		return time.UnixMilli(int64(v)).Format("2006-01-02")
	default:
		return ""
	}
}
