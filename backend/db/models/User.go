package models

type User struct {
	ID string `gorm:"primaryKey"`
	// Name     string `validate:"required,min=2"` // TODO add this when profile is supported
	Email string `gorm:"uniqueIndex" validate:"required,email"`
	// Tasks []Task
}

type AuthUser struct {
	LocalID string `json:"localId"` // uid
	Email   string `json:"email"`
	// TODO add profile
}

type FirebaseLoginResponse struct {
	LocalID      string `json:"localId"` // uid
	Email        string `json:"email"`
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    string `json:"expiresIn"`
}

type FirebaseRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    string `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	UserID       string `json:"user_id"`
	ProjectID    string `json:"project_id"`
}
