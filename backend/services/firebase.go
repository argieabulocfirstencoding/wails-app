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
	// TODO add keyring implementation to store sensitive information
	//TODO add local account copy on successful login

	return &result, nil
}

func FirebaseRequest(ctx context.Context, idToken, path, method string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/(default)/%s?key=%s",
		config.FIREBASE_PROJECT_ID, path, config.FIREBASE_API)

	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, url, buf)
	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("firestore error %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
