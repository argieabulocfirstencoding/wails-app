package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kbinani/screenshot"
)

func Screenshot() ([]string, error) {
	n := GetMonitorCount()

	outputDir, err := filepath.Abs(filepath.Join("backend", "screenshots"))
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

	return pictures, nil
}

func GetMonitorCount() int {
	n := screenshot.NumActiveDisplays()
	fmt.Printf("You have %d displays\n", n)

	return n
}

func OpenFile(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Use "explorer" for Windows
		cmd = exec.Command("explorer", path)
	case "darwin":
		// Use "open" for macOS
		cmd = exec.Command("open", path)
	default:
		// Use "xdg-open" for Linux
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}
