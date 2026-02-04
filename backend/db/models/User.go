package models

type User struct {
	ID         string `gorm:"primaryKey"`
	Name       string `gorm:"default:''"`
	FirstName  string `gorm:"default:''"`
	LastName   string `gorm:"default:''"`
	MiddleName string `gorm:"default:''"`
	Position   string `gorm:"default:''"`
	Birthdate  string `gorm:"default:''"`
	Phone      string `gorm:"default:''"`
	Address    string `gorm:"default:''"`
	Photo      string `gorm:"default:''"`
	Email      string `gorm:"default:''"`
	FESID      string `gorm:"default:''"`
	Attendance []Attendance
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
