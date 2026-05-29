package service_classes

import (
	"bytes"
	"context"
	"encoding/json"
	config "fesapp/backend"
	db "fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"os/exec"
	"time"

	"github.com/go-vgo/robotgo/clipboard"
	"github.com/micmonay/keybd_event"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
	"golang.org/x/sys/windows"

	native_runtime "runtime"
)

type FesClientService struct {
	baseUrl        string
	client         *http.Client
	logger         *utils.LoggerService
	TaskRepository *repository.TaskRepository
	ctx            context.Context
	apiToken       string
}

func NewFesClientService() *FesClientService {

	baseUrl := "https://firstencoding.app/api/1.1/"
	_, exists := os.LookupEnv("DEVELOPMENT")
	if exists {
		baseUrl = "https://firstencoding.app/version-test/api/1.1/"
	}

	return &FesClientService{
		// baseUrl: "https://firstencoding.app/version-test/api/1.1/",
		baseUrl:        baseUrl,
		client:         &http.Client{Timeout: 30 * time.Second},
		logger:         utils.NewLoggerService(),
		TaskRepository: repository.NewTaskRepository(),
	}

}

// ============== GENERAL METHODS =======================

func (f *FesClientService) getApiToken() string {
	if f.apiToken != "" {
		return f.apiToken
	}

	if apiToken, err := keyring.Get("credentials", "fes_api_token"); err == nil {
		f.apiToken = apiToken
		return apiToken
	}

	return ""
}

func (f *FesClientService) SetApiToken(token string) {
	f.apiToken = token
	keyring.Set("credentials", "fes_api_token", token)
}

func (f *FesClientService) Request(request types.Request) (map[string]interface{}, error) {
	apiToken := f.getApiToken()

	url := f.baseUrl + request.Url
	reqType := request.Method

	// Convert bodyData to JSON
	jsonBody, err := json.Marshal(request.Body)
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

	res, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	rawBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		errMsg := http.StatusText(res.StatusCode)

		var parsed map[string]any
		if len(rawBody) > 0 && json.Unmarshal(rawBody, &parsed) == nil {
			if msg, ok := parsed["message"].(string); ok && msg != "" {
				errMsg = msg
			}
		} else if trimmed := strings.TrimSpace(string(rawBody)); trimmed != "" {
			errMsg = trimmed
		}

		return nil, fmt.Errorf("fes api request failed (%d): %s", res.StatusCode, errMsg)
	}

	var result map[string]any
	if err := json.Unmarshal(rawBody, &result); err != nil {
		f.logger.Printlnf("Error decoding response: %v", err)
		return nil, err
	}

	f.logger.Printlnf("RESULT: %v", result)
	response, ok := result["response"].(map[string]any)
	if !ok {
		errorMessage, ok := result["message"].(string)
		if !ok {
			return nil, f.logger.Error("failed to parse login response: unexpected format")
		}
		return nil, f.logger.Errorf("%s", errorMessage)
	}

	return response, nil
}

// ========================== UTILITY ===============================

func (f *FesClientService) OpenFesWebApp() error {
	if !config.OnlineMode {
		return f.logger.Error("you are offline")
	}

	if resp, err := f.Request(types.Request{
		Url:    "wf/open_web_app",
		Method: "GET",
		Body:   map[string]any{},
	}); err == nil {
		url, ok := resp["magic_link"].(string)
		if !ok {
			return f.logger.Errorf("Error on opening fes web app")
		}

		if err := f.openIncognitoBrowserUrl(url); err != nil {
			return err
		}

	} else {
		return err
	}

	return nil
}

// ========================== CLIENTS & PROJECTS ===============================
func (f *FesClientService) SyncClientsAndProjects() error {
	if err := f.fetchClients(); err != nil {
		return err
	}

	if err := f.fetchProjects(); err != nil {
		return err
	}

	return nil
}

// FetchClients retrieves clients in bubble and store them directly in the database
func (f *FesClientService) fetchClients() error {
	apiToken := f.getApiToken()

	fmt.Println("Fetching FES Client Lists")
	targetUrl := f.baseUrl + "obj/clients?cursor="
	cursor := 0

	var clients []db.Client

	for {
		url := fmt.Sprintf("%s%d", targetUrl, cursor)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to get request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error when on request :%w", err)
		}
		defer res.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			return fmt.Errorf("error on decoding response body: %w", err)
		}

		response, ok := result["response"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("error on getting response value")
		}

		results, ok := response["results"].([]interface{})
		if !ok || len(response) == 0 {
			return fmt.Errorf("error on getting results value")
		}

		remaining, ok := response["remaining"].(float64)
		if !ok {
			return fmt.Errorf("error no remaining value was found")
		}

		clients = make([]db.Client, 0)
		for _, item := range results {

			record, ok := item.(map[string]interface{})

			if !ok {
				continue
			}

			id, _ := record["_id"].(string)
			fullName, _ := record["Full name"].(string)
			companyName, _ := record["Company Name"].(string)

			clients = append(clients, db.Client{
				ID:          id,
				CompanyName: companyName,
				Fullname:    fullName,
			})
		}

		// Creating the current loop batch of clients
		if err := f.TaskRepository.BulkCreateClients(clients); err != nil {
			return fmt.Errorf("error on creating clients: %w", err)
		}

		// Increment search cursor to another 100 since bubble data api limit is 100 only
		cursor = cursor + 100

		// If there is no remaining records then stop the loop
		if remaining == 0 {
			break
		}
	}

	return nil
}

func (f *FesClientService) fetchProjects() error {
	apiToken := f.getApiToken()

	fmt.Printf("Fetching FES Projects")
	targetUrl := f.baseUrl + "obj/projects-projects?limit=100&cursor="
	cursor := 0
	var tasks []db.Task

	for {
		url := fmt.Sprintf("%s%d", targetUrl, cursor)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer res.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode response %w", err)
		}

		response, ok := result["response"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid response format")
		}

		results, ok := response["results"].([]interface{})
		if !ok || len(results) == 0 {
			return fmt.Errorf("no results found")
		}

		remaining, ok := response["remaining"].(float64)
		if !ok {
			return fmt.Errorf("no remaining value indicated")
		}

		tasks = make([]db.Task, 0, len(results))

		for _, item := range results {
			record, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			status, _ := record["Status"].(string)

			// Checks if the project is not available then skip it
			if status == "End of contract" {
				continue
			}

			id, _ := record["_id"].(string)
			clientId, _ := record["Client"].(string)
			account, _ := record["Account"].(string)
			projectName, _ := record["Project name"].(string)

			tasks = append(tasks, db.Task{
				TaskID:      id,
				Title:       account,
				ClientID:    clientId,
				ProjectName: projectName,
			})
		}

		// Creating the current loop batch of tasks
		if err := f.TaskRepository.BulkCreateTasks(tasks); err != nil {
			utils.Logger.Printf("error on bulk task create %v", err)
			return err
		}

		// Increment search cursor to another 100 since bubble data api limit is 100 only
		cursor = cursor + 100

		// If there is no remaining records then stop the loop
		if remaining == 0 {
			break
		}
	}

	return nil
}

func (f *FesClientService) FetchUpworkCreds(ctx context.Context, taskID string) {
	if !config.OnlineMode {
		return
	}

	fmt.Print("Fetching Upwork Credentials Please wait")
	f.openFESApp()
	time.Sleep(1000 * time.Millisecond)
	f.logger.Print("Opening upwork..")
	// openFESApp(ctx)

	runtime.EventsEmit(ctx, "upwork-button", true)

	task, err := f.TaskRepository.GetTaskFromTaskID(taskID)
	if err != nil {
		log.Printf("error on getting task: %v", err)
	}

	url := "wf/fetch-credentials/"
	body := map[string]any{
		"account": task.Title,
	}
	response, _ := f.Request(types.Request{
		Method: "POST",
		Url:    url,
		Body:   body,
	})

	copyToClipboard := func(key string) error {
		if valueInterface, ok := response[key]; ok {
			if value, ok := valueInterface.(string); ok {
				if value == "" {
					return fmt.Errorf("error copy")
				}
				clipboard.WriteAll(value)
			} else {
				// Handle the case where the value is not a string
				fmt.Println(key + " is not a string")
				clipboard.WriteAll("")
				return fmt.Errorf("error copy")
			}
		} else {
			fmt.Println(key + " key not found")
			clipboard.WriteAll("")
			return fmt.Errorf("error copy")
		}
		return nil
	}

	go func() {
		kb, _ := keybd_event.NewKeyBonding()

		// Copy email
		if err := copyToClipboard("password"); err != nil {
			return
		}
		time.Sleep(1000 * time.Millisecond)

		// Press Ctrl + V
		kb.SetKeys(keybd_event.VK_V)
		kb.HasCTRL(true)
		// Copy password
		if err := copyToClipboard("username"); err != nil {
			return
		}
		kb.HasCTRL(false)

		time.Sleep(1000 * time.Millisecond)

		f.openFESApp()
		runtime.EventsEmit(ctx, "upwork-button", true)
	}()
}

// ============================ PRIVATES ========================

func (f *FesClientService) openFESApp() {
	verb := "open"
	file := "fesapp://"

	// 1. Convert strings to UTF16 pointers and handle the potential error
	verbPtr, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return // or log the error
	}

	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return // or log the error
	}

	// 2. Pass the pointers directly to ShellExecute
	// We don't need [0] anymore because we have the actual pointer variable
	_ = windows.ShellExecute(0, verbPtr, filePtr, nil, nil, windows.SW_SHOWNORMAL)
}

func (f *FesClientService) openIncognitoBrowserUrl(url string) error {
	var cmd *exec.Cmd

	switch native_runtime.GOOS {
	case "windows":
		// Windows: use 'start' or call chrome directly
		// Note: If chrome isn't in PATH, you may need the full path
		cmd = exec.Command("cmd", "/c", "start", "chrome", "--incognito", url)
	case "darwin":
		// macOS: uses the 'open' command with the -a flag for the app name
		cmd = exec.Command("open", "-na", "Google Chrome", "--args", "--incognito", url)
	case "linux":
		// Linux: usually 'google-chrome' or 'google-chrome-stable'
		cmd = exec.Command("google-chrome", "--incognito", url)
	default:
		return f.logger.Errorf("Unsupported OS")
	}

	err := cmd.Start()
	if err != nil {
		return f.logger.Errorf("Error opening Chrome: %s", err.Error())
	}

	return nil
}
