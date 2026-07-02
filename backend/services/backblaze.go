package services

import (
	"context"
	"fesapp/backend/db/models"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kurin/blazer/b2"
	"github.com/zalando/go-keyring"
)

type BackblazeSync struct {
	ctx         context.Context
	isConnected bool
	connecting  bool
	AuthUser    *models.AuthUser
	Bucket      *b2.Bucket

	queue []func()
	mu    sync.RWMutex
}

const backblazeEnabled = true

func NewBackblazeSync() *BackblazeSync {
	return &BackblazeSync{}
}

func (b *BackblazeSync) Enabled() bool {
	return backblazeEnabled
}

func (b *BackblazeSync) SetContext(ctx context.Context) {
	if ctx == nil {
		return
	}

	b.mu.Lock()
	b.ctx = ctx
	b.mu.Unlock()
}

func (b *BackblazeSync) SetAuthUser(authUser *models.AuthUser) {
	b.mu.Lock()
	b.AuthUser = authUser
	b.mu.Unlock()
}

func (b *BackblazeSync) Connect(ctx context.Context) error {
	if !backblazeEnabled {
		b.mu.Lock()
		b.ctx = ctx
		b.isConnected = false
		b.Bucket = nil
		b.mu.Unlock()
		return nil
	}

	b.mu.Lock()
	if ctx != nil {
		b.ctx = ctx
	}
	if b.isConnected && b.Bucket != nil {
		b.mu.Unlock()
		return nil
	}
	if b.connecting {
		b.mu.Unlock()
		return fmt.Errorf("backblaze connection already in progress")
	}
	b.connecting = true
	b.mu.Unlock()

	var connectErr error
	defer func() {
		b.mu.Lock()
		b.connecting = false
		if connectErr != nil {
			b.isConnected = false
			b.Bucket = nil
		}
		b.mu.Unlock()
	}()

	accountID, err := keyring.Get("credentials", "APP_ID")
	if err != nil {
		connectErr = fmt.Errorf("read backblaze app id: %w", err)
		return connectErr
	}
	applicationKey, err := keyring.Get("credentials", "APP_KEY")
	if err != nil {
		connectErr = fmt.Errorf("read backblaze app key: %w", err)
		return connectErr
	}

	if accountID == "" || applicationKey == "" {
		connectErr = fmt.Errorf("backblaze credentials are not configured")
		return connectErr
	}

	connectCtx, cancel := context.WithTimeout(b.connectionContext(), 30*time.Second)
	defer cancel()

	client, err := b2.NewClient(connectCtx, accountID, applicationKey)
	if err != nil {
		connectErr = fmt.Errorf("failed to create backblaze client: %w", err)
		return connectErr
	}

	bucket, err := client.Bucket(connectCtx, "fesapp-screenshots")
	if err != nil {
		connectErr = fmt.Errorf("failed to get backblaze bucket: %w", err)
		return connectErr
	}

	b.mu.Lock()
	b.Bucket = bucket
	b.isConnected = true
	b.mu.Unlock()
	log.Printf("connected to backblaze successfully")

	return nil
}

func (b *BackblazeSync) AttemptUpload(paths []string) ([]string, error) {
	return b.attemptUpload(paths, "", 0)
}

func (b *BackblazeSync) AttemptUploadToFolder(paths []string, folderName string, screenshotID uint) ([]string, error) {
	return b.attemptUpload(paths, folderName, screenshotID)
}

func (b *BackblazeSync) attemptUpload(paths []string, folderName string, screenshotID uint) ([]string, error) {
	if !backblazeEnabled {
		log.Println("Backblaze upload disabled for this release")
		return nil, nil
	}
	if len(paths) == 0 {
		return nil, nil
	}

	authUser := b.getAuthUser()
	if authUser == nil || strings.TrimSpace(authUser.LocalID) == "" {
		return nil, fmt.Errorf("backblaze upload requires an authenticated user")
	}

	if err := b.ensureConnected(); err != nil {
		return nil, err
	}

	return b.upload(paths, authUser.LocalID, folderName, screenshotID)
}

func (b *BackblazeSync) upload(paths []string, userID, folderName string, screenshotID uint) ([]string, error) {
	if !backblazeEnabled {
		return nil, nil
	}

	b.mu.RLock()
	bucket := b.Bucket
	uploadCtx := b.ctx
	b.mu.RUnlock()

	if bucket == nil {
		return nil, fmt.Errorf("backblaze bucket is not initialized")
	}
	if uploadCtx == nil {
		uploadCtx = context.Background()
	}

	remotePaths := make([]string, 0, len(paths))
	for _, fullpath := range paths {
		file, err := os.Open(fullpath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", fullpath, err)
		}

		remotePath := buildRemoteObjectPath(userID, folderName, screenshotID, fullpath)
		obj := bucket.Object(remotePath)
		writer := obj.NewWriter(uploadCtx)

		if _, err := io.Copy(writer, file); err != nil {
			_ = writer.Close()
			_ = file.Close()
			return nil, fmt.Errorf("failed to upload %s: %w", fullpath, err)
		}

		if err := writer.Close(); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("failed to finalize upload %s: %w", fullpath, err)
		}

		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("failed to close file %s: %w", fullpath, err)
		}

		remotePaths = append(remotePaths, remotePath)
	}

	log.Print("Succesfully synced screenshots")
	return remotePaths, nil
}

func (b *BackblazeSync) ensureConnected() error {
	b.mu.RLock()
	connected := b.isConnected && b.Bucket != nil
	b.mu.RUnlock()

	if connected {
		return nil
	}

	return b.Connect(nil)
}

func (b *BackblazeSync) getAuthUser() *models.AuthUser {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.AuthUser
}

func (b *BackblazeSync) connectionContext() context.Context {
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()

	if ctx != nil {
		return ctx
	}

	return context.Background()
}

func buildRemoteObjectPath(userID, folderName string, screenshotID uint, localPath string) string {
	filename := strings.TrimSpace(filepath.Base(localPath))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		filename = "screenshot.webp"
	}

	if screenshotID > 0 {
		filename = fmt.Sprintf("%d-%s", screenshotID, filename)
	}

	folderName = sanitizeBackblazeSegment(folderName)
	if folderName != "" {
		return path.Join("screenshots", userID, folderName, filename)
	}

	parent := strings.TrimSpace(filepath.Base(filepath.Dir(localPath)))

	if parent == "" || parent == "." || parent == string(filepath.Separator) {
		return path.Join("screenshots", userID, filename)
	}

	return path.Join("screenshots", userID, parent, filename)
}

func sanitizeBackblazeSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func (b *BackblazeSync) attemptUploadQueued(paths []string) ([]string, error) {
	if !backblazeEnabled {
		log.Println("Backblaze upload disabled for this release")
		return nil, nil
	}

	b.mu.Lock()
	if !b.isConnected {
		log.Println("⚠️ Not connected — queuing upload")

		// Attempts to reconnect
		go func() {
			if err := b.Connect(context.Background()); err != nil {
				log.Printf("error connecting reason: %v", err)
			}
		}()

		// Queue the job (runs in background later)
		b.queue = append(b.queue, func() {
			if _, err := b.uploadQueued(paths); err != nil {
				log.Printf("Upload failed for queued job: %v", err)
			}
		})

		b.mu.Unlock()
		// Return nil because upload did not happen yet
		return nil, nil
	}
	b.mu.Unlock()

	remotePaths, err := b.uploadQueued(paths)
	return remotePaths, err
}

func (b *BackblazeSync) uploadQueued(paths []string) ([]string, error) {
	if !backblazeEnabled {
		return nil, nil
	}

	remotePaths := []string{}

	for _, fullpath := range paths {
		filename := filepath.Base(fullpath)
		remotePath := fmt.Sprintf("screenshots/%s/%s", b.AuthUser.LocalID, filename)

		file, err := os.Open(fullpath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", fullpath, err)
		}

		obj := b.Bucket.Object(remotePath)
		writer := obj.NewWriter(b.ctx)

		if _, err := io.Copy(writer, file); err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to upload: %w", err)
		}

		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("failed to finalize upload: %w", err)
		}

		file.Close()
		remotePaths = append(remotePaths, remotePath)
	}

	log.Print("Succesfully synced screenshots")
	return remotePaths, nil
}
