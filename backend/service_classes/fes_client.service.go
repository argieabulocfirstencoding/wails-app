package service_classes

import (
	"bytes"
	"encoding/json"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"net/http"
	"os"
)

type FesClientService struct {
	baseUrl string
	client  *http.Client
	logger  *utils.LoggerService
}

func NewFesClientService() *FesClientService {

	return &FesClientService{
		// baseUrl:     "https://firstencoding.app/api/1.1/wf/",
		baseUrl: "https://firstencoding.app/version-test/api/1.1/wf/",
		client:  &http.Client{},
		logger:  utils.NewLoggerService(),
	}
}

func (f *FesClientService) getApiToken() string {
	return os.Getenv("FES_API") // FIX this is only for testing purpose,
}

func (f *FesClientService) Request(request types.Request) (map[string]interface{}, error) {
	apiToken := f.getApiToken()

	url := f.baseUrl + request.Pathname
	reqType := request.ReqType

	f.logger.Printlnf("Request: %v", request)

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

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		f.logger.Printlnf("Error decoding response: %v", err)
		return nil, err
	}

	f.logger.Printlnf("Response: %v", result)

	// _, ok := result["response"].(map[string]interface{})
	// if !ok {
	// 	f.logger.Println("Error on parsing response")
	// 	return nil, err
	// }

	return result, nil
}
