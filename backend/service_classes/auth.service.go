package serviceclasses

import "fesapp/backend/db/models"

type AuthService struct {
	SyncService *FesSyncService

	baseUrl string
}

func NewAuthService() *AuthService {
	syncService := NewFesSyncService()
	return &AuthService{
		SyncService: syncService,
		baseUrl: "https://firstencoding.app/api/1.1/wf/",
	}
}

func (a *AuthService) Login(email, password string) (*models.Attendance, *models.AuthUser, error) {

	authUser, err := a.getAuthUser(email, password)
	if err != nil {
		return nil, nil, err
	}

	attendance, err := a.attemptLogin(email, password)
	if err != nil {
		return nil, nil, err
	}

	return attendance, authUser, nil
}

func (a *AuthService) Logout(email string) error {
	return nil
}

//  ======================= PRIVATE METHODS ===========================

func (a *AuthService) attemptLogin(email, password string) (*models.Attendance, error) {

	
	// TODO add attempt login logic to fes app
	return nil, nil
}

func (a *AuthService) getAuthUser(email, password string) (*models.AuthUser, error) {
	// TODO get auth user logic from fes app
	return nil, nil
}


