package services

import (
	"context"
	"fesapp/backend/db/models"
	"log"
	"time"
)

type SyncService struct {
	ctx         context.Context
	isConnected bool
	AuthUser    *models.FirebaseLoginResponse
	idToken     *string
}

func NewSyncService() *SyncService {
	return &SyncService{}
}

func (s *SyncService) Connect(ctx context.Context, authUser *models.FirebaseLoginResponse) error {
	s.ctx = ctx
	s.AuthUser = authUser
	s.isConnected = CheckFirebaseConnection()
	if !s.isConnected {
		// retry
	}
	idToken, err := FirebaseUpdateAccessToken(s.ctx, s.AuthUser.Email)
	if err != nil {
		return err
	}
	s.idToken = idToken
	return nil
}

func (s *SyncService) Reconnect() error {
	s.isConnected = CheckFirebaseConnection()
	idToken, err := FirebaseUpdateAccessToken(s.ctx, s.AuthUser.Email)
	if err != nil {
		return err
	}
	s.idToken = idToken
	return nil
}

func (s *SyncService) TestSync() {
	path := "documents/testcol/asdasdasd"
	body := map[string]interface{}{
		"fields": map[string]interface{}{
			"name": map[string]string{
				"stringValue": "Hello firestore!",
			},
			"count": map[string]string{
				"integerValue": "42",
			},
			"createdAt": map[string]string{
				"timestampValue": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	if s.isConnected {
		// TODO for now it always update access token on refresh make this base on expiresAt field
		idToken, err := FirebaseUpdateAccessToken(s.ctx, s.AuthUser.Email)
		if err != nil {
			log.Printf("failed to refresh token: %v", err)
			return
		}

		resp, err := FirebaseDocumentRequest(s.ctx, *idToken, path, "PATCH", body)
		if err != nil {
			log.Printf("firestore write failed: %v", err)
		}

		log.Println("Firestore sync response:", string(resp))
	}
}

func (s *SyncService) SyncTasks() {
	path := "documents/tasks"
	method := "GET"

	if s.isConnected {
		// TODO for now it always update access token on refresh make this base on expiresAt field
		idToken, err := FirebaseUpdateAccessToken(s.ctx, s.AuthUser.Email)
		if err != nil {
			log.Printf("failed to refresh token: %v", err)
			return
		}

		resp, err := FirebaseDocumentRequest(s.ctx, *idToken, path, method, nil)
		if err != nil {
			log.Printf("firestore write failed: %v", err)
		}

		log.Println("Firestore sync response:", string(resp))
	}
}
