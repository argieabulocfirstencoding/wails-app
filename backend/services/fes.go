package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fesapp/backend/db/models"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	// alias to avoid conflict with Wails runtime
	stdruntime "runtime" // alias to avoid conflict with Wails runtime

	"github.com/go-vgo/robotgo/clipboard"
	"github.com/micmonay/keybd_event"
)

// TODO WORK ON THIS, GET TASKS IN BUBBLE

func FetchTasksFromFES() ([]models.ExpTask, error) {
	apiToken := os.Getenv("FES_API")
	if apiToken == "" {
		return nil, fmt.Errorf("FES_API environment variable not set")
	}

	url := "https://firstencoding.app/api/1.1/wf/working-projects"
	// url := "https://firstencoding.app/version-test/api/1.1/wf/working-projects"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer res.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	response, ok := result["response"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	clientsResults, ok := response["Client"].([]interface{})

	if !ok || len(clientsResults) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	clients := make([]models.Client, 0, len(clientsResults))
	for _, item := range clientsResults {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := record["_id"].(string)
		fullName, _ := record["Full name"].(string)
		companyName, _ := record["Company Name"].(string)

		clients = append(clients, models.Client{
			ID:          id,
			CompanyName: companyName,
			Fullname:    fullName,
		})
	}

	if err := BulkCreateClients(clients); err != nil {
		return nil, err
	}

	results, ok := response["Projects"].([]interface{})
	if !ok || len(results) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	tasks := make([]models.Task, 0, len(results))

	for _, item := range results {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := record["_id"].(string)
		clientId, _ := record["Client"].(string)
		account, _ := record["Account"].(string)

		tasks = append(tasks, models.Task{
			TaskID:   id,
			Title:    account,
			ClientID: clientId,
		})
	}

	if err := BulkCreateTasks(tasks); err != nil {
		return nil, err
	}

	expTasks := models.ToExpTasks(tasks)

	return expTasks, nil
}

func FetchUpworkCreds(ctx context.Context) {
	fmt.Print("Fetching Upwork Credentials Pleas wait")
	url := "https://firstencoding.app/version-test/api/1.1/wf/fetch-credentials/"
	body := map[string]interface{}{
		"account": "Upwork (Frederick)",
	}
	response, _ := FetchBubble(url, "POST", body)

	copyToClipboard := func(key string) {
		if valueInterface, ok := response[key]; ok {
			if value, ok := valueInterface.(string); ok {
				clipboard.WriteAll(value)
			} else {
				// Handle the case where the value is not a string
				fmt.Println(key + " is not a string")
			}
		} else {
			fmt.Println(key + " key not found")
		}
	}

	kb, _ := keybd_event.NewKeyBonding()

	// Copy email
	copyToClipboard("password")
	time.Sleep(1000 * time.Millisecond)

	// Press Ctrl + V
	kb.SetKeys(keybd_event.VK_V)
	kb.HasCTRL(true)
	// Copy password
	copyToClipboard("username")
	fmt.Print("Fetching Upwork Credentials Successful")

	time.Sleep(1000 * time.Millisecond)
	fmt.Print("Opening upwork..")
	err := openFESApp()
	if err != nil {
		fmt.Print(err)
	}
}

func openFESApp() error {
	url := "fesapp://"

	switch stdruntime.GOOS {
	case "windows":
		// start the URL using the default registered protocol
		cmd := exec.Command("cmd", "/C", "start", url)
		return cmd.Run()
	case "darwin":
		cmd := exec.Command("open", url)
		return cmd.Run()
	case "linux":
		cmd := exec.Command("xdg-open", url)
		return cmd.Run()
	default:
		return nil
	}
}

func FetchBubble(url, reqType string, bodyData map[string]interface{}) (map[string]interface{}, error) {
	apiToken := os.Getenv("FES_API")

	// Convert bodyData to JSON
	jsonBody, err := json.Marshal(bodyData)
	if err != nil {
		return nil, err
	}

	// Create request with body
	req, err := http.NewRequest(reqType, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json") // important for JSON

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	response, ok := result["response"].(map[string]interface{})
	if !ok {
		log.Fatalf("Invalid response format")
		return nil, err
	}

	// Do something with 'response' if needed
	fmt.Println(response)

	return response, nil
}
