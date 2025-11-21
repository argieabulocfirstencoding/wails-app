package services

import (
	"context"
	"encoding/json"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
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
	IdToken     *string
	Backblaze   *BackblazeSync

	connecting bool
	queue      []func()
	mu         sync.Mutex
}

func NewSyncService() *SyncService {
	backblaze := NewBackblazeSync()
	return &SyncService{
		Backblaze: backblaze,
	}
}

func (s *SyncService) Connect(ctx context.Context) error {
	s.ctx = ctx
	s.isConnected = false
	s.connecting = true

	s.isConnected = CheckFirebaseConnection()
	if !s.isConnected {
		return fmt.Errorf("no connection established")
	}

	s.mu.Lock()
	queued := s.queue
	s.queue = nil
	s.mu.Unlock()

	for _, job := range queued {
		job()
	}

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
		s.IdToken = idToken
		s.AuthUser.IDToken = *idToken
		s.AuthUser.ExpiresIn = *expiresIn
	}

	s.isConnected = true
}

// NOTE this is is made only for testing purposes not needed in production
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
		resp, err := FirebaseDocumentRequest(s.ctx, *s.IdToken, path, "PATCH", body)
		if err != nil {
			log.Printf("firestore write failed: %v", err)
		}

		log.Println("Firestore sync response:", string(resp))
	}
}

func (s *SyncService) SyncCredentials() {
	path := "documents/creds?pageSize=1"
	method := "GET"

	s.RefreshAccessToken()
	if s.isConnected {
		resp, err := FirebaseDocumentRequest(s.ctx, *s.IdToken, path, method, nil)
		if err != nil {
			utils.Logger.Printf("failed fetching important keys: %v", err)

			time.Sleep(3000 * time.Millisecond)
			s.SyncCredentials()
		}

		var firestoreResponse map[string]interface{}
		if err := json.Unmarshal(resp, &firestoreResponse); err != nil {
			utils.Logger.Printf("failed to decode firestore response on creds: %v", err)
		}

		docs, ok := firestoreResponse["documents"].([]interface{})
		if !ok || len(docs) == 0 {
			utils.Logger.Printf("cannot find documents on firestore")
			return
		}

		// Get the first document
		firstDoc, ok := docs[0].(map[string]interface{})
		if !ok {
			utils.Logger.Printf("failed casting first document")
			return
		}

		// Access fields
		fields, ok := firstDoc["fields"].(map[string]interface{})
		if !ok {
			utils.Logger.Printf("cannot find fields in the first document")
			return
		}

		APP_KEY := getStringValue(fields["APP_KEY"])
		APP_ID := getStringValue(fields["APP_ID"])
		FES_API := getStringValue(fields["FES_API"])

		fmt.Printf("APP_KEY: %s", APP_KEY)
		fmt.Printf("APP_ID: %s", APP_ID)
		fmt.Printf("FES_API: %s", FES_API)
		// Store in keyring
		keyring.Set("credentials", "APP_KEY", APP_KEY)
		keyring.Set("credentials", "APP_ID", APP_ID)
		keyring.Set("credentials", "FES_API", FES_API)
	}
}

// NOTE this is deprecated tasks are from bubble api now
func (s *SyncService) SyncTasks() {
	path := "documents/tasks"
	method := "GET"

	s.RefreshAccessToken()
	if s.isConnected {
		resp, err := FirebaseDocumentRequest(s.ctx, *s.IdToken, path, method, nil)
		if err != nil {
			log.Printf("firestore write failed: %v", err)
		}
		// log.Println("Firestore sync response:", string(resp))

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
				"task_id = ?",
				uid, s.AuthUser.LocalID,
			).Attrs(models.Task{
				TaskID:   uid,
				ClientID: client,
				Title:    title,
			}).FirstOrCreate(&task).Error; err != nil {
				log.Printf("error syncing task: %v", err)
			}
		}
	}
}

// ================== QUEUE SYNC HELPER ========================
func (s *SyncService) AttemptSyncTaskRecord(taskRecord *models.TaskRecord) error {
	s.mu.Lock()
	if !s.isConnected {
		log.Println("⚠️ Not connected — queuing syncing task record")

		// Attempts to reconnect
		go func() {
			if err := s.Connect(context.Background()); err != nil {
				log.Printf("error connecting reason: %v", err)
			}
		}()

		// Queue the job (runs in background later)
		s.queue = append(s.queue, func() {
			log.Printf("Queueing syncing task record waiting for connection")
			if err := s.SyncTaskRecord(taskRecord); err != nil {
				log.Printf("Syncin task record failed for queued job: %v", err)
			}
		})

		s.mu.Unlock()
		// Return nil because syncing did not happen yet
		return nil
	}
	s.mu.Unlock()

	err := s.SyncTaskRecord(taskRecord)
	return err
}

func (s *SyncService) AttemptSyncAttendance(attendance *models.Attendance) error {
	s.mu.Lock()
	if !s.isConnected {
		log.Println("⚠️ Not connected — queuing syncing attendance")
		go func() {
			if err := s.Connect(context.Background()); err != nil {
				log.Printf("error connecting reason: %v", err)
			}
		}()

		s.queue = append(s.queue, func() {
			log.Printf("Queueing syncing attendance waiting for connection")
			if err := s.SyncAttendance(attendance); err != nil {
				log.Printf("Syncin attendance failed for queued job: %v", err)
			}
		})
		s.mu.Unlock()
		return nil
	}

	s.mu.Unlock()
	err := s.SyncAttendance(attendance)
	return err
}

// ================== ACTUAL FUNCTIONS ========================

func (s *SyncService) SyncTaskRecord(taskRecord *models.TaskRecord) error {
	// ============== PRE DEFINITION =============
	now := time.Now()
	recordID := fmt.Sprintf("%d", now.UTC().UnixMilli())
	attendance, err := getAttendanceFromID(taskRecord.AttendanceID)
	if err != nil {
		return err
	}
	task, err := getTaskFromID(taskRecord.TaskID)
	if err != nil {
		return err
	}

	attendanceFormattedID := attendance.CreatedAt.UTC().Format("20060102T150405")
	// NOTE if failed to end task, use synced_at as ended_at
	fields := map[string]interface{}{
		"synced_at": map[string]string{
			"timestampValue": now.UTC().Format(time.RFC3339),
		},
		"task_id": map[string]string{
			"stringValue": taskRecord.TaskID,
		},
		"task_name": map[string]string{
			"stringValue": task.Client.Fullname + " - " + task.Title,
		},
		"started_at": map[string]string{
			"timestampValue": taskRecord.StartedAt.UTC().Format(time.RFC3339),
		},
		"ended_at": map[string]string{
			"timestampValue": now.UTC().Format(time.RFC3339),
		},
		"duration": map[string]string{
			"integerValue": fmt.Sprintf("%d", int(now.Sub(taskRecord.StartedAt).Seconds())),
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
	path := fmt.Sprintf("documents/users/%s/attendance/%s/tasks/%s/task_records/%s",
		taskRecord.UserID, attendanceFormattedID, taskRecord.TaskID, recordID,
	)
	method := "PATCH"
	body := map[string]interface{}{
		"fields": fields,
	}
	s.RefreshAccessToken()
	if s.isConnected {
		_, err := FirebaseDocumentRequest(s.ctx, *s.IdToken, path, method, body)
		if err != nil {
			return err
		}

		now := time.Now()
		taskRecord.SyncedAt = &now
		db.DB.Save(&taskRecord)
	}

	// updates database that record is synced
	db.DB.Model(&taskRecord).Update("sync_status", "synced")
	db.DB.Model(&attendance).Update("sync_status", "pending")

	if err = s.AttemptSyncAttendance(attendance); err != nil {
		return err
	}

	return nil
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) error {
	// ============== PRE DEFINITION =============
	now := time.Now()
	attendanceFormattedID := attendance.CreatedAt.UTC().Format("20060102T150405")
	s.RefreshAccessToken()

	fields := map[string]interface{}{
		"synced_at": map[string]string{
			"timestampValue": now.UTC().Format(time.RFC3339),
		},
		"started_at": map[string]string{
			"timestampValue": attendance.TimeIn.UTC().Format(time.RFC3339),
		},
		"duration": map[string]string{
			"integerValue": fmt.Sprintf("%d", int(now.Sub(attendance.TimeIn).Seconds())),
		},
	}

	if attendance.TimeOut != nil {
		fields["ended_at"] = map[string]string{
			"timestampValue": attendance.TimeOut.UTC().Format(time.RFC3339),
		}
		fields["duration"] = map[string]string{
			"integerValue": fmt.Sprintf("%d", int(attendance.TimeOut.Sub(attendance.TimeIn).Seconds())),
		}
	}
	// ============= REQUEST CONFIG ==============
	path := fmt.Sprintf("documents/users/%s/attendance/%s/",
		attendance.UserID, attendanceFormattedID,
	)
	method := "PATCH"
	body := map[string]interface{}{
		"fields": fields,
	}

	if s.isConnected {
		_, err := FirebaseDocumentRequest(s.ctx, *s.IdToken, path, method, body)
		if err != nil {
			return err
		}

		now := time.Now()
		attendance.SyncedAt = &now
		attendance.SyncStatus = "synced"
		db.DB.Save(&attendance)
	}

	return nil
}

func (s *SyncService) SyncAllAttendance() error {
	var unsynced []models.Attendance
	if err := db.DB.Where("user_id = ? and synced_at IS NULL").Find(&unsynced).Error; err != nil {
		return err
	}
	// TODO get the attendance record in firebase that is currently active or not yet concluded.
	// Change the local copy of start time to the ones in firebase.
	// Sync all local task records to firebase
	// How to know if the

	//TODO new way
	// Sync Attendance creates a summary of how many working hours(attendance), total tasks time, idle times, breaks, in duration as a summary
	// or consider adding a summary when the start and end of tasks as well as it's

	if s.isConnected {
		for _, attendance := range unsynced {
			if err := s.SyncAttendance(&attendance); err != nil {
				log.Printf("error syncing task_record: %v", err)
				return err
			}
		}
	}

	return nil
}

func (s *SyncService) SyncUnsyncedRecords() error {
	var unsynced []models.TaskRecord
	if err := db.DB.Where("user_id = ? AND sync_status = 'pending'", s.AuthUser.LocalID).
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
			return err
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
