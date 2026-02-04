package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	// alias to avoid conflict with Wails runtime
	// alias to avoid conflict with Wails runtime

	"github.com/go-vgo/robotgo/clipboard"
	"github.com/micmonay/keybd_event"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
)

// TODO WORK ON THIS, GET TASKS IN BUBBLE
// FIX this is deprecated
func FetchTasksFromFES() ([]models.ExpTask, error) {
	apiToken, err := keyring.Get("credentials", "FES_API")
	if err != nil {
		return nil, err
	}
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

func SyncClientsAndProjects() error {
	if err := FetchClients(); err != nil {
		return err
	}

	if err := FetchProjects(); err != nil {
		return err
	}

	return nil
}

// FetchClients retrieves clients in bubble and store them directly in the database
func FetchClients() error {
	apiToken, err := keyring.Get("credentials", "FES_API")
	if err != nil {
		return err
	}
	fmt.Println("Fetching FES Client Lists")
	targetUrl := "https://firstencoding.app/api/1.1/obj/clients?cursor="
	cursor := 0

	var clients []models.Client

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

		clients = make([]models.Client, 0)
		for _, item := range results {

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

		// Creating the current loop batch of clients
		if err := BulkCreateClients(clients); err != nil {
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

func FetchProjects() error {
	apiToken, err := keyring.Get("credentials", "FES_API")
	if err != nil {
		return err
	}

	fmt.Printf("Fetching FES Projects")
	targetUrl := "https://firstencoding.app/api/1.1/obj/projects-projects?limit=100&cursor="
	cursor := 0
	var tasks []models.Task

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

		tasks = make([]models.Task, 0, len(results))

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

			tasks = append(tasks, models.Task{
				TaskID:      id,
				Title:       account,
				ClientID:    clientId,
				ProjectName: projectName,
			})
		}

		// Creating the current loop batch of tasks
		if err := BulkCreateTasks(tasks); err != nil {
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

// TODO continue
func FetchEmployeeDetails(ctx context.Context, user *models.User) error {
	// https://firstencoding.app/version-test/api/1.1/wf/employee_details

	// FIX development mode only
	url := "https://firstencoding.app/api/1.1/wf/employee_details"
	method := "POST"

	email := user.Email
	if user.Email == "test@fes.com" {
		email = "johnpaultunacao.firstencoding@gmail.com"
	}

	body := map[string]interface{}{
		"email": email,
	}

	response, err := FetchBubble(url, method, body)
	if err != nil {
		return err
	}

	employeeRaw, exists := response["employee"]
	if !exists || employeeRaw == nil {
		return nil
	}

	switch v := employeeRaw.(type) {
	case map[string]interface{}:
		employee := v

		fesid, _ := response["fesid"].(string)
		rawBirthdate, _ := employee["BirthDate txt"].(string)

		birthdate := ""
		if rawBirthdate != "" {
			slicedBd := strings.Split(rawBirthdate, " ")

			if len(slicedBd) >= 3 {
				birthdate = fmt.Sprintf("%s %s %s", slicedBd[0], slicedBd[1], slicedBd[2])
			}

		}
		fmt.Println("Birthdate:", rawBirthdate)

		if fesid != "" {
			fields := map[string]string{
				user.Name:       employee["Full Name"].(string),
				user.FirstName:  employee["FirstName"].(string),
				user.LastName:   employee["LastName"].(string),
				user.MiddleName: employee["MiddleName"].(string),
				user.FESID:      response["fesid"].(string),
				user.Photo:      response["photo"].(string),
				user.Position:   response["position"].(string),
				user.Phone:      employee["Account phone number"].(string),
				user.Address:    employee["Address"].(string),
				user.Birthdate:  birthdate,
			}

			updated := false
			for ptr, val := range fields {
				updated = checkIfNeedUpdate(ptr, val) || updated
			}

			if updated {
				if err := db.DB.Save(&user).Error; err != nil {
					fmt.Println("Failed to save user:", err)
				}
			}
		}

		return nil
		// use employee
	case []interface{}:
		// employee is a list
		return nil
	default:
		// unexpected type
		return nil
	}

}

func FetchUpworkCreds(ctx context.Context, taskID string) {
	fmt.Print("Fetching Upwork Credentials Pleas wait")
	time.Sleep(1000 * time.Millisecond)
	fmt.Print("Opening upwork..")

	openFESApp(ctx)
	runtime.EventsEmit(ctx, "upwork-button", true)

	task, err := GetTaskFromID(taskID)
	if err != nil {
		log.Printf("error on getting task: %v", err)
	}

	url := "https://firstencoding.app/api/1.1/wf/fetch-credentials/"
	body := map[string]interface{}{
		"account": task.Title,
	}
	response, _ := FetchBubble(url, "POST", body)

	copyToClipboard := func(key string) {
		if valueInterface, ok := response[key]; ok {
			if value, ok := valueInterface.(string); ok {
				clipboard.WriteAll(value)
			} else {
				// Handle the case where the value is not a string
				fmt.Println(key + " is not a string")
				clipboard.WriteAll("")
				return
			}
		} else {
			fmt.Println(key + " key not found")
			clipboard.WriteAll("")
			return
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

	openFESApp(ctx)
	runtime.EventsEmit(ctx, "upwork-button", true)
}

func openFESApp(ctx context.Context) {
	runtime.BrowserOpenURL(ctx, "fesapp://")
}

func FetchBubble(url, reqType string, bodyData map[string]interface{}) (map[string]interface{}, error) {
	apiToken, err := keyring.Get("credentials", "FES_API")
	if err != nil {
		return nil, err
	}

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

	return response, nil
}

// Updates a **string pointer** if nil or value changed, returns true if updated
func checkIfNeedUpdate(field string, val string) bool {
	if val == "" {
		return false // skip empty updates
	}

	if field == "" || field != val {
		field = val
		return true
	}

	return false
}
