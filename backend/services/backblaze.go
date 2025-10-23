package services

import (
	"context"
	"fesapp/backend/db/models"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/kurin/blazer/b2"
)

type BackblazeSync struct {
	ctx         context.Context
	isConnected bool
	AuthUser    *models.FirebaseLoginResponse
	Bucket      *b2.Bucket

	queue []func()
	mu    sync.Mutex
}

func NewBackblazeSync() *BackblazeSync {
	return &BackblazeSync{}
}

func (b *BackblazeSync) Connect(ctx context.Context) error {
	b.ctx = ctx
	b.isConnected = false

	accountID := "005e8f8e51d56660000000005"
	applicationKey := "K005NokfI7e0kE0qMhz0CCTHkAB4r3A"
	bucketName := "fesapp-screenshots" // Replace with your bucket name

	client, err := b2.NewClient(ctx, accountID, applicationKey)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	bucket, err := client.Bucket(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get bucket: %w", err)
	}

	b.Bucket = bucket

	b.isConnected = true
	log.Printf("connected to backblaze successfully")

	// Process any queued uploads
	b.mu.Lock()
	queued := b.queue
	b.queue = nil
	b.mu.Unlock()

	for _, job := range queued {
		job()
	}

	return nil
}

func (b *BackblazeSync) AttemptUpload(paths []string) ([]string, error) {
	b.mu.Lock()
	if !b.isConnected || b.Bucket == nil {
		log.Println("⚠️ Not connected — queuing upload")
		// Queue the job (runs in background later)
		b.queue = append(b.queue, func() {
			if _, err := b.upload(paths); err != nil {
				log.Printf("Upload failed for queued job: %v", err)
			}
		})

		b.mu.Unlock()
		// Return nil because upload did not happen yet
		return nil, nil
	}
	b.mu.Unlock()

	remotePaths, err := b.upload(paths)
	return remotePaths, err
}

func (b *BackblazeSync) upload(paths []string) ([]string, error) {
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
