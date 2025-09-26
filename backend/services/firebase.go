package services

import (
	"bytes"
	"context"
	"encoding/json"
	config "fesapp/backend"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zalando/go-keyring"
)

type FirebaseLoginResponse struct {
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    string `json:"expiresIn"`
	LocalID      string `json:"localId"` // uid
	Email        string `json:"email"`
}

func CheckFirebaseConnection() bool {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.googleapis.com/generate_204")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 204
}

func FirebaseLogin(email, password string) (*FirebaseLoginResponse, error) {
	// NOTE public api key for authenticating not security problem

	url := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + config.FIREBASE_API

	payload := map[string]string{
		"email":             email,
		"password":          password,
		"returnSecureToken": "true",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("firebase auth failed: %s", resp.Status)
	}

	var result FirebaseLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	keyring.Set("refresh", result.Email, result.RefreshToken)
	CreateUser(result.LocalID, result.Email)
	// TODO add update as well the time when the user logs in

	//TODO add local account copy on successful login

	return &result, nil
}

func FirebaseUpdateAccessToken(ctx context.Context, email string) (*string, error) {

	refreshToken, err := keyring.Get("refresh", email)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://securetoken.googleapis.com/v1/token?key=%s", config.FIREBASE_API)
	body := map[string]string{
		"grantType":    "refreshToken",
		"refreshToken": refreshToken,
	}
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}
	result, err := FirebaseRequest(ctx, url, "POST", body, headers)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Acquiring new Id Token: %v", result)
	test := "testing"

	// NOTE testing
	return &test, nil
}

func FirebaseRequest(ctx context.Context, url, method string, body interface{}, headers map[string]string) ([]byte, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, buf)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("firestore error %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func FirebaseDocumentRequest(ctx context.Context, idToken, path, method string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/(default)/%s?key=%s",
		config.FIREBASE_PROJECT_ID, path, config.FIREBASE_API)

	headers := map[string]string{
		"Authorization": "Bearer " + idToken,
		"content-type":  "application/json",
	}

	return FirebaseRequest(ctx, url, method, body, headers)
}
