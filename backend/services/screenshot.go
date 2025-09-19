package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func Screenshot(folderName string) ([]string, error) {
	n := GetMonitorCount()

	outputDir, err := filepath.Abs(filepath.Join("backend", "screenshots", folderName))
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		panic(err)
	}

	pictures := []string{}

	// Loop through all displays and save screenshot
	timestamp := time.Now().Format("20060102_150405")
	for i := 0; i < n; i++ {

		bounds := screenshot.GetDisplayBounds(i)

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return nil, err
		}

		fileName := filepath.Join(outputDir, fmt.Sprintf("screenshot_%s_display%d.png", timestamp, i))
		file, err := os.Create(fileName)
		if err != nil {
			return nil, err
		}

		if err := png.Encode(file, img); err != nil {
			file.Close()
			return nil, err
		}
		file.Close()

		fmt.Printf("Saved %s\n", fileName)

		// Convert to base64 for preview
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		base64Img := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

		pictures = append(pictures, base64Img)
	}

	screenshotNotify()
	return pictures, nil
}

func screenshotNotify() {
	exePath, _ := os.Executable()
	root := filepath.Dir(exePath)
	iconPath := filepath.Join(root, "icon.ico")

	//NOTE Icon is duplicated and placed in build/bin/ and build/windows/
	beeep.Notify("Screenshot", "Your work progress have been captured", iconPath)
}

func GetMonitorCount() int {
	n := screenshot.NumActiveDisplays()
	fmt.Printf("You have %d displays\n", n)

	return n
}

type ScreenshotService struct {
	ctx      context.Context
	stopChan chan struct{}
	running  bool
}

func NewScreenshotService() *ScreenshotService {
	return &ScreenshotService{
		stopChan: make(chan struct{}),
	}
}

func (s *ScreenshotService) Start(ctx context.Context) {
	s.ctx = ctx

	const TIME_TO_SCREENSHOT_IN_MINUTES time.Duration = 10
	folderName := time.Now().Format("20060102_150405")
	if s.running {
		return
	}
	s.running = true

	fmt.Println("Started Screenshot Tracking")
	go func() {
		ticker := time.NewTicker(time.Minute * TIME_TO_SCREENSHOT_IN_MINUTES)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// TODO: take screenshot here
				screenshot, err := Screenshot(folderName)
				if err != nil {
					log.Fatal(err)
				}

				runtime.EventsEmit(s.ctx, "screenshot", screenshot)
				fmt.Println("📸 Taking screenshot...")
			case <-s.stopChan:
				fmt.Println("Stopping screenshot service...")
				s.running = false
				return
			}
		}
	}()

}

func (s *ScreenshotService) Stop() {
	fmt.Println("Stopped Screenshot Tracking")
	if s.running {
		s.stopChan <- struct{}{}
	}
}
