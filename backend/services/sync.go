package services

import (
	"context"
	"encoding/json"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

type FirestoreDocument struct {
	Name   string                 `json:"name"`
	Fields map[string]interface{} `json:"fields"`
}

type FirestoreListResponse struct {
	Documents []FirestoreDocument `json:"documents"`
}

func ExtractUIDFromPath(docName string) string {
	parts := strings.Split(docName, "/")
	return parts[len(parts)-1]
}

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
	// s.isConnected = CheckFirebaseConnection()
	s.isConnected = true
	if !s.isConnected {
		return fmt.Errorf("no connection established")
	}
	s.idToken = &authUser.IDToken
	return nil
}

func (s *SyncService) Reconnect() error {
	// TODO do a reconnect attempt 5 times
	s.isConnected = CheckFirebaseConnection()
	s.RefreshAccessToken()
	return nil
}

func (s *SyncService) RefreshAccessToken() {
	// Refresh Access Token only if it has expired
	if FirebaseAccessTokenIsExpired(s.AuthUser.ExpiresIn) {
		idToken, expiresIn, err := FirebaseUpdateAccessToken(s.ctx, s.AuthUser.Email)
		if err != nil {
			log.Printf("failed to refresh token: %v", err)
			return
		}
		s.idToken = idToken
		s.AuthUser.IDToken = *idToken
		s.AuthUser.ExpiresIn = *expiresIn
	}
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
		resp, err := FirebaseDocumentRequest(s.ctx, *s.idToken, path, "PATCH", body)
		if err != nil {
			log.Printf("firestore write failed: %v", err)
		}

		log.Println("Firestore sync response:", string(resp))
	}
}

func (s *SyncService) SyncTasks() {
	path := "documents/tasks"
	method := "GET"

	s.RefreshAccessToken()
	if s.isConnected {
		resp, err := FirebaseDocumentRequest(s.ctx, *s.idToken, path, method, nil)
		if err != nil {
			log.Printf("firestore write failed: %v", err)
		}

		log.Println("Firestore sync response:", string(resp))

		var firestoreResponse FirestoreListResponse
		if err := json.Unmarshal(resp, &firestoreResponse); err != nil {
			log.Printf("error on unpacking firestore respone: %v", err)
		}

		for _, doc := range firestoreResponse.Documents {
			uid := ExtractUIDFromPath(doc.Name)
			client := getStringValue(doc.Fields["client"])
			title := getStringValue(doc.Fields["title"])

			var task models.Task
			if err := db.DB.Where(
				"task_id = ? AND user_id = ?",
				uid, s.AuthUser.LocalID,
			).Attrs(models.Task{
				TaskID: uid,
				Client: client,
				Title:  title,
				UserID: s.AuthUser.LocalID,
			}).FirstOrCreate(&task).Error; err != nil {
				log.Printf("error syncing task: %v", err)
			}
		}
	}
}

func (s *SyncService) SyncTaskRecord(taskRecord *models.TaskRecord) error {
	// ============== PRE DEFINITION =============
	startedAt := taskRecord.StartedAt.UTC().Format("20060102T150405")
	recordID := taskRecord.UserID + "_" + taskRecord.TaskID + "_" + startedAt

	fields := map[string]interface{}{
		"task_id": map[string]string{
			"stringValue": taskRecord.TaskID,
		},
		"started_at": map[string]string{
			"timestampValue": taskRecord.StartedAt.UTC().Format(time.RFC3339),
		},
	}
	if taskRecord.EndedAt != nil {
		fields["ended_at"] = map[string]string{
			"timestampValue": taskRecord.EndedAt.UTC().Format(time.RFC3339),
		}
	}
	if taskRecord.Duration != nil {
		fields["duration"] = map[string]string{
			"integerValue": fmt.Sprintf("%d", *taskRecord.Duration),
		}
	}

	// ============= REQUEST CONFIG ==============
	path := fmt.Sprintf("documents/users/%s/tasks/%s/task_records/%s",
		taskRecord.UserID, taskRecord.TaskID, recordID,
	)
	method := "PATCH"
	body := map[string]interface{}{
		"fields": fields,
	}
	s.RefreshAccessToken()
	if s.isConnected {
		_, err := FirebaseDocumentRequest(s.ctx, *s.idToken, path, method, body)
		if err != nil {
			return err
		}

		now := time.Now()
		taskRecord.SyncedAt = &now
		db.DB.Save(&taskRecord)
	}

	return nil
}

func (s *SyncService) SyncAttendance() error {
	var unsynced []models.Attendance
	if err := db.DB.Where("user_id = ? and synced_at IS NULL").Find(&unsynced).Error; err != nil {
		return err
	}
	// TODO get the attendance record in firebase that is currently active or not yet concluded.
	// Change the local copy of start time to the ones in firebase.
	// Sync all local task records to firebase
	// How to know if the 

	// ============= REQUEST CONFIG ==============
	// path := ""
	// method := "PATCH"
	// body := map[string]interface{}{
	// 	"fields": fields,
	// }
	s.RefreshAccessToken()

	if s.isConnected {
		// _, err := FirebaseDocumentRequest(s.ctx, *s.idToken, path, method, body)
		// if err != nil {
		// 	return err
		// }

		// now := time.Now()
		// taskRecord.SyncedAt = &now
		// db.DB.Save(&taskRecord)
	}

	return nil
}

func (s *SyncService) SyncUnsyncedRecords() error {
	var unsynced []models.TaskRecord
	if err := db.DB.Where("user_id = ? AND synced_at IS NULL OR (ended_at IS NOT NULL AND synced_at < ended_at)", s.AuthUser.LocalID).
		Find(&unsynced).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Print("no unsynced records")
			return nil
		} else {
			return err
		}
	}

	for _, record := range unsynced {
		if err := s.SyncTaskRecord(&record); err != nil {
			log.Printf("error syncing task_record: %v", err)
		}
	}

	return nil
}

func getStringValue(field interface{}) string {
	if field == nil {
		return ""
	}
	if m, ok := field.(map[string]interface{}); ok {
		if v, ok := m["stringValue"].(string); ok {
			return v
		}
	}
	return ""
}

// helper to extract integerValue
func getIntValue(field interface{}) int {
	if field == nil {
		return 0
	}
	if m, ok := field.(map[string]interface{}); ok {
		if v, ok := m["integerValue"].(string); ok {
			var num int
			fmt.Sscanf(v, "%d", &num)
			return num
		}
	}
	return 0
}
