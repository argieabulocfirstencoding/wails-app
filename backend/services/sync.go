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
	"strconv"
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

type FirestoreTimestamp struct {
	TimestampValue string `json:"timestampValue"`
}
type FirestoreString struct {
	StringValue string `json:"stringValue"`
}
type FirestoreInteger struct {
	IntegerValue string `json:"integerValue"`
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
	IssuedAt    *time.Time
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

	// --- CRITICAL SECTION 1: Check and Set Connecting Flag ---
	s.mu.Lock()
	if s.connecting {
		s.mu.Unlock()
		return nil
	}
	s.connecting = true
	s.mu.Unlock()

	// Defer ensures 'connecting' is always reset.
	defer func() {
		s.mu.Lock()
		s.connecting = false
		s.mu.Unlock()
	}()

	// --- NETWORK CALL (Blocking/Retries) ---
	// This call now handles all 5 retries internally.
	testResult := s.TestSync()

	// --- CRITICAL SECTION 2: Update State and Process Queue ---
	s.mu.Lock()
	s.isConnected = testResult

	if !s.isConnected {
		// s.TestSync failed after all retries.
		s.mu.Unlock()
		return fmt.Errorf("no connection established after 5 attempts")
	}

	// If connected, process queued jobs.
	queued := s.queue
	s.queue = nil
	s.mu.Unlock()

	// --- Execute Jobs (Non-Critical Section) ---
	// The previous sync tasks are now executed because the connection is up.
	for _, job := range queued {
		// Jobs run concurrently to ensure fast processing of the queue
		go job()
	}

	return nil
}

func (s *SyncService) RefreshAccessToken() {
	// Refresh Access Token only if it has expired
	isExpired := FirebaseAccessTokenIsExpired(s.AuthUser.ExpiresIn, *s.IssuedAt)
	// isExpired := true

	utils.Logger.Printf("access token status: %t", isExpired)
	if !isExpired {
		s.isConnected = true
		return
	}

	idToken, expiresIn, err := FirebaseUpdateAccessToken(s.ctx, s.AuthUser.Email)
	if err != nil {
		utils.Logger.Printf("failed to refresh token: %v", err)
		s.isConnected = false
		return
	}

	utils.Logger.Println("successfully regenerated firebase access token")

	now := time.Now()
	s.isConnected = true
	s.IdToken = idToken
	s.AuthUser.IDToken = *idToken
	s.AuthUser.ExpiresIn = *expiresIn
	s.IssuedAt = &now

	seconds, err := strconv.ParseInt(*expiresIn, 10, 64)
	if err != nil {
		utils.Logger.Printf("failed to parse expiresIn: %v", err)
	} else {
		expTime := time.Now().Add(time.Duration(seconds) * time.Second)
		utils.Logger.Printf("current token expires at: %s", expTime.Format(time.RFC3339))
	}
}

func (s *SyncService) TestSync() bool {
	const maxRetries = 3
	// Start with a short delay and let it grow exponentially
	baseDelay := 500 * time.Millisecond
	testPath := "users"

	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("Attempting to connect to firebase (Attempt %d/%d)...\n", attempt, maxRetries)

		// *** CORE REQUEST ***
		_, err := FirebaseDocumentRequest(s.ctx, *s.IdToken, testPath, "GET", nil)

		// 1. SUCCESS: If successful, return true immediately.
		if err == nil {
			fmt.Println("Successfully connected to Firebase.")
			return true
		}

		// Log the failure and error for debugging (RECOMMENDED)
		log.Printf("Firebase connection check failed (Attempt %d): %v", attempt, err)

		// 2. FAILURE ON FINAL ATTEMPT: If it failed on the final attempt, return false.
		if attempt == maxRetries {
			fmt.Println("Failed to connect to Firebase after all retries.")
			return false // <--- CORRECTED: Returns FALSE on final failure
		}

		// 3. CALCULATE EXPONENTIAL BACKOFF DELAY
		// Calculate the delay: baseDelay * 2^(attempt-1). Cap it at a max.
		delay := baseDelay << (attempt - 1)
		maxDelay := 30 * time.Second
		if delay > maxDelay {
			delay = maxDelay
		}

		// 4. WAIT: Use the select statement to wait OR exit early on context cancellation.
		fmt.Printf("Waiting for %v before next retry...\n", delay)
		select {
		case <-time.After(delay):
			// Wait time finished, proceed to next iteration (loop continues)
		case <-s.ctx.Done():
			// Context was canceled (e.g., app shutdown). Exit immediately.
			log.Println("Connection check interrupted by context cancellation.")
			return false
		}
	}

	// Should theoretically not be reached with the new logic, but kept for completeness
	return false
}

func (s *SyncService) SyncCredentials() {
	path := "creds?pageSize=1"
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

		APP_KEY := utils.GetStringValue(fields["APP_KEY"])
		APP_ID := utils.GetStringValue(fields["APP_ID"])
		FES_API := utils.GetStringValue(fields["FES_API"])

		// utils.Logger.Printf("APP_KEY: %s", APP_KEY)
		// utils.Logger.Printf("APP_ID: %s", APP_ID)
		// utils.Logger.Printf("FES_API: %s", FES_API)

		// Store in keyring
		keyring.Set("credentials", "APP_KEY", APP_KEY)
		keyring.Set("credentials", "APP_ID", APP_ID)
		keyring.Set("credentials", "FES_API", FES_API)
	}
}

// NOTE this is deprecated tasks are from bubble api now
func (s *SyncService) SyncTasks() {
	path := "tasks"
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
			client := utils.GetStringValue(doc.Fields["client"])
			title := utils.GetStringValue(doc.Fields["title"])

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
	if taskRecord.SyncStatus == "synced" {
		return nil
	}

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
	fmt.Println("Synced Task Record")
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
	db.DB.Model(&taskRecord).Update("sync_status", "pending")
	// ============== PRE DEFINITION =============
	now := time.Now()
	user, err := GetUserFromId(taskRecord.UserID)
	if err != nil {
		return fmt.Errorf("error in getting user data from synctaskrecord %s", err)
	}
	attendance, err := GetAttendanceFromID(taskRecord.AttendanceID)
	if err != nil {
		return err
	}
	task, err := GetTaskFromID(taskRecord.TaskID)
	if err != nil {
		return err
	}

	duration := int(now.Sub(taskRecord.StartedAt).Seconds())

	// NOTE if failed to end task, use synced_at as ended_at
	fields := map[string]interface{}{
		"synced_at": FirestoreInteger{
			IntegerValue: strconv.FormatInt(now.Local().UnixMilli(), 10),
		},
		"attendance_date": FirestoreInteger{
			IntegerValue: strconv.FormatInt(attendance.TimeIn.Local().UnixMilli(), 10),
		},
		"task_id": map[string]string{
			"stringValue": taskRecord.TaskID,
		},
		"padagan": map[string]bool{
			"booleanValue": taskRecord.Padagan,
		},
		"task_name": map[string]string{
			"stringValue": task.Client.Fullname + " - " + task.Title,
		},
		"SupervisorID": FirestoreString{
			StringValue: user.SupervisorId,
		},
		"EmployeeID": FirestoreString{
			StringValue: user.EmployeeId,
		},
		"EmploymentID": FirestoreString{
			StringValue: user.EmploymentId,
		},
		"started_at": FirestoreInteger{
			IntegerValue: strconv.FormatInt(taskRecord.StartedAt.Local().UnixMilli(), 10),
		},
		"ended_at": FirestoreInteger{
			IntegerValue: strconv.FormatInt(now.Local().UnixMilli(), 10),
		},
		"duration": map[string]string{
			"integerValue": fmt.Sprintf("%d", duration),
		},
	}

	db.DB.Model(&taskRecord).Update("duration", duration)

	if taskRecord.EndedAt != nil {
		fields["ended_at"] = FirestoreInteger{
			IntegerValue: strconv.FormatInt(now.Local().UnixMilli(), 10),
		}
	}

	// ============= REQUEST CONFIG ==============
	path := fmt.Sprintf("task_records/%s-%d",
		user.EmployeeId, taskRecord.StartedAt.Local().UnixMilli(),
	)
	// path := fmt.Sprintf("users/%s/attendance/%s/tasks/%s/task_records/%s",
	// 	taskRecord.UserID, attendanceFormattedID, taskRecord.TaskID, recordID,
	// )
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
	db.DB.Model(&taskRecord).Updates(map[string]interface{}{
		"sync_status": "synced",
		"synced_at":   time.Now(),
	})

	if err = s.SyncAttendance(attendance); err != nil {
		return err
	}

	return nil
}

func (s *SyncService) SyncUser(user models.AuthUser) error {
	now := time.Now()
	userModel, err := GetUserFromId(user.LocalID)
	if err != nil {
		return err
	}

	s.RefreshAccessToken()

	middleInitial := ""
	if len(userModel.MiddleName) > 0 {
		middleInitial = fmt.Sprintf("%c.", userModel.MiddleName[0])
	}

	fields := map[string]interface{}{
		"FESID": FirestoreString{
			StringValue: userModel.FESID,
		},
		"Full Name": FirestoreString{
			StringValue: fmt.Sprintf("%s %s %s", userModel.FirstName, middleInitial, userModel.LastName),
		},
		"First Name": FirestoreString{
			StringValue: userModel.FirstName,
		},
		"Last Name": FirestoreString{
			StringValue: userModel.LastName,
		},
		"Middle Name": FirestoreString{
			StringValue: userModel.MiddleName,
		},
		"SupervisorID": FirestoreString{
			StringValue: userModel.SupervisorId,
		},
		"EmployeeID": FirestoreString{
			StringValue: userModel.EmployeeId,
		},
		"EmploymentID": FirestoreString{
			StringValue: userModel.EmploymentId,
		},
		"email": FirestoreString{
			StringValue: user.Email,
		},
		"updated_at": FirestoreTimestamp{
			TimestampValue: now.UTC().Format(time.RFC3339),
		},
	}

	registerPath := fmt.Sprintf("users/%s", user.LocalID)
	method := "PATCH"
	body := map[string]interface{}{
		"fields": fields,
	}

	if s.isConnected {
		_, err := FirebaseDocumentRequest(s.ctx, *s.IdToken, registerPath, method, body)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) error {
	db.DB.Model(&attendance).Update("sync_status", "pending")
	user, err := GetUserFromId(attendance.UserID)
	if err != nil {
		return fmt.Errorf("error in syncattendance when fetching user %s", err)
	}
	// ============== PRE DEFINITION =============
	now := time.Now()
	s.RefreshAccessToken()
	CalculateAttendanceSummary(attendance)

	fields := map[string]interface{}{
		"synced_at": FirestoreInteger{
			IntegerValue: strconv.FormatInt(now.Local().UnixMilli(), 10),
		},
		"started_at": FirestoreInteger{
			IntegerValue: strconv.FormatInt(attendance.TimeIn.Local().UnixMilli(), 10),
		},
		"duration": FirestoreInteger{
			IntegerValue: fmt.Sprintf("%d", int(now.Sub(attendance.TimeIn).Seconds())),
		},
		"client_time": FirestoreInteger{
			IntegerValue: fmt.Sprintf("%d", attendance.ClientTime),
		},
		"total_gap": FirestoreInteger{
			IntegerValue: fmt.Sprintf("%d", attendance.TotalGap),
		},
		"SupervisorID": FirestoreString{
			StringValue: user.SupervisorId,
		},
		"EmployeeID": FirestoreString{
			StringValue: user.EmployeeId,
		},
		"EmploymentID": FirestoreString{
			StringValue: user.EmploymentId,
		},
	}

	if attendance.TimeOut != nil {
		duration := int(attendance.TimeOut.Sub(attendance.TimeIn).Seconds())
		fields["ended_at"] = FirestoreInteger{
			IntegerValue: strconv.FormatInt(attendance.TimeOut.Local().UnixMilli(), 10),
		}
		fields["duration"] = FirestoreInteger{
			IntegerValue: fmt.Sprintf("%d", duration),
		}
	}

	// ============= REQUEST CONFIG ==============
	path := fmt.Sprintf("attendance/%s-%d",
		user.EmployeeId, attendance.TimeIn.Local().UnixMilli(),
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

		db.DB.Model(&attendance).Updates(map[string]interface{}{
			"sync_status": "synced",
			"synced_at":   time.Now(),
		})

		// Sync all unsynced records of the current attendance session
		if err := s.SyncUnsyncedRecords(); err != nil {
			log.Println("Not able to sync previous task records")
		}
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
	attendance, err := GetUserAttendance(s.AuthUser.LocalID)
	if err != nil {
		return err
	}

	var unsynced []models.TaskRecord
	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND sync_status = 'pending'", s.AuthUser.LocalID, attendance.ID).
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
