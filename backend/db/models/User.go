package models

type User struct {
	ID         string `gorm:"primaryKey"`
	Email      string `gorm:"uniqueIndex" validate:"required,email"`
	Name       *string
	FirstName  *string
	LastName   *string
	MiddleName *string
	Position   *string
	Birthdate  *string
	Phone      *string
	Address    *string
	Photo      *string
	FESID      *string `gorm:"uniqueIndex"`
	// Tasks []Task
}

type AuthUser struct {
	LocalID   string `json:"localId"` // uid
	Email     string `json:"email"`
	Name      string `json:"name"`
	Position  string `json:"position"`
	Birthdate string `json:"birthdate"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
	Photo     string `json:"photo"`
	FESID     string `json:"fesid"`
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
