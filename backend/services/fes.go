package services

import (
	"encoding/json"
	"fesapp/backend/db/models"
	"fmt"
	"log"
	"net/http"
	"os"
)

// TODO WORK ON THIS, GET TASKS IN BUBBLE

func FetchTasksFromFES() ([]models.ExpTask, error) {
	apiToken := os.Getenv("FES_API")
	if apiToken == "" {
		return nil, fmt.Errorf("FES_API environment variable not set")
	}

	url := "https://firstencoding.app/version-test/api/1.1/wf/working-projects"
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

func FetchBubbleData(targetData, reqType string) error {
	apiToken := os.Getenv("FES_API")
	url := "https://yourapp.bubbleapps.io/api/1.1/obj/" + targetData
	req, _ := http.NewRequest(reqType, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiToken)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)

	response, ok := result["response"].(map[string]interface{})
	if !ok {
		log.Fatalf("Invalid response format")
		return err
	}

	results, ok := response["results"].([]interface{})
	if !ok || len(results) == 0 {
		log.Fatalf("No results found")
		return err
	}

	var tasks []models.ExpTask

	for _, item := range results {
		firstItem := item.(map[string]interface{})

		// Extract fields
		id := firstItem["_id"].(string)
		fullName := firstItem["Full name"].(string)
		companyName := firstItem["Company Name"].(string)

		tasks = append(tasks, models.ExpTask{
			TaskID: id,
			Title:  companyName,
			Client: fullName,
		})
	}

	fmt.Printf("taks: %v", tasks)

	return nil
}
