package services

import (
	"bytes"
	"context"
	"encoding/json"
	config "fesapp/backend"
	"fesapp/backend/db/models"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

func CheckFirebaseConnection() bool {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.googleapis.com/generate_204")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 204
}

func FirebaseLogin(email, password string) (*models.FirebaseLoginResponse, error) {
	// NOTE public api key for authenticating not security problem

	URL := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + config.FIREBASE_API

	payload := map[string]string{
		"email":             email,
		"password":          password,
		"returnSecureToken": "true",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(URL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("firebase auth failed: %s", resp.Status)
	}

	var result models.FirebaseLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	keyring.Set("refresh", result.Email, result.RefreshToken)
	CreateUser(result.LocalID, result.Email)
	// TODO add update as well the time when the user logs in

	//TODO add local account copy on successful login

	return &result, nil
}

func FirebaseUpdateAccessToken(ctx context.Context, email string) (idToken *string, err error) {
	refreshToken, err := keyring.Get("refresh", email)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("https://securetoken.googleapis.com/v1/token?key=%s", config.FIREBASE_API)

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to refresh token: %s", string(bodyBytes))
	}

	var out models.FirebaseRefreshResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return nil, err
	}

	if out.RefreshToken != "" {
		_ = keyring.Set("refresh", email, out.RefreshToken)
	}

	return &out.IDToken, nil
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
	url := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/(default)/%s",
		config.FIREBASE_PROJECT_ID, path)

	headers := map[string]string{
		"Authorization": "Bearer " + idToken,
		"Content-Type":  "application/json",
	}

	return FirebaseRequest(ctx, url, method, body, headers)
}
