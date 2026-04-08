package service_classes

import (
	"bytes"
	"encoding/json"
	"fesapp/backend/types"
	"log"
	"net/http"

	"github.com/zalando/go-keyring"
)

type FesClientService struct {
	baseUrl  string
	client   *http.Client
	apiToken string
	logger   LoggerService
}

func NewFesClientService() *FesClientService {
	return &FesClientService{
		baseUrl: "https://firstencoding.app/api/1.1/wf/",
		client:  &http.Client{},
		logger:  *NewLoggerService(),
	}
}

func (f *FesClientService) Request(request types.Request) (map[string]interface{}, error) {
	apiToken, err := keyring.Get("credentials", "FES_API")
	if err != nil {
		f.logger.Print(err.Error())

		return nil, err
	}

	url := f.baseUrl + request.Pathname
	reqType := request.ReqType

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
