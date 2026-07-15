package service_classes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	db "fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"os/exec"
	"time"

	"github.com/go-vgo/robotgo/clipboard"
	"github.com/gorilla/websocket"
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
	Connectivity   *ConnectivityStateService
	ctx            context.Context
	apiToken       string
}

type ClientAccount struct {
	ID               string `json:"id"`
	Display          string `json:"display"`
	Platform         string `json:"platform"`
	Holder           string `json:"holder"`
	HasCredentials   bool   `json:"hasCredentials"`
	GoogleProfile    string `json:"googleProfile"`
	HasGoogleProfile bool   `json:"hasGoogleProfile"`
}

type clientAccountRecord struct {
	ID            string
	Display       string
	Platform      string
	Holder        string
	Username      string
	Password      string
	GoogleProfile string
}

type upworkCredentials struct {
	Username string
	Password string
}

type chromeDevToolsTarget struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type chromeDevToolsMessage struct {
	ID     int                  `json:"id,omitempty"`
	Method string               `json:"method,omitempty"`
	Params map[string]any       `json:"params,omitempty"`
	Result json.RawMessage      `json:"result,omitempty"`
	Error  *chromeDevToolsError `json:"error,omitempty"`
}

type chromeDevToolsError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type chromeRuntimeEvaluateResult struct {
	Result struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	} `json:"result"`
	ExceptionDetails *struct {
		Text string `json:"text"`
	} `json:"exceptionDetails,omitempty"`
}

type chromeDevToolsClient struct {
	conn   *websocket.Conn
	nextID int
}
type chromeLocalState struct {
	Profile struct {
		InfoCache map[string]chromeProfileInfo `json:"info_cache"`
	} `json:"profile"`
}

type chromeProfileInfo struct {
	UserName     string `json:"user_name"`
	Name         string `json:"name"`
	ShortcutName string `json:"shortcut_name"`
	GaiaName     string `json:"gaia_name"`
}

const (
	upworkInitialFocusDelay              = 1750 * time.Millisecond
	upworkClientAccountInitialFocusDelay = 4 * time.Second
	upworkClientAccountPasswordDelay     = 2500 * time.Millisecond
	upworkClientAccountSubmitDelay       = 500 * time.Millisecond
	upworkClipboardClearDelay            = 300 * time.Millisecond
	upworkFieldTransitionDelay           = 350 * time.Millisecond
	upworkFieldReadyTimeout              = 20 * time.Second
	upworkDevToolsTargetTimeout          = 8 * time.Second
	upworkFieldReadyPollInterval         = 250 * time.Millisecond
	upworkChromeRemoteDebuggingPort      = 9333
	upworkChromeLoginURL                 = "https://www.upwork.com/ab/account-security/login?redir=%2Fab%2Faccount-security%2Foauth2%2Fauthorize%3Fresponse_type%3Dcode%26redirect_uri%3Dupwork%253A%252F%252Fwww.upwork.com%252Foauth2redir%26client_id%3D522767a70366bdd00e95cd6e8756940c"
)

var (
	upworkUsernameFieldSelectors = []string{
		`input#login_username`,
		`input[name="login[username]"]`,
		`input[name="username"]`,
		`input[autocomplete="username"]`,
		`input[type="email"]`,
	}
	upworkPasswordFieldSelectors = []string{
		`input#login_password`,
		`input[name="login[password]"]`,
		`input[name="password"]`,
		`input[autocomplete="current-password"]`,
		`input[type="password"]`,
	}
)

func NewFesClientService(connectivity ...*ConnectivityStateService) *FesClientService {
	var connectivityState *ConnectivityStateService
	if len(connectivity) > 0 {
		connectivityState = connectivity[0]
	}
	if connectivityState == nil {
		connectivityState = NewConnectivityStateService()
	}

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
		Connectivity:   connectivityState,
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

		return nil, types.NewRemoteAPIError("FES API", res.StatusCode, errMsg)
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

// ========================== ENVIRONMENTS ==========================

func (f *FesClientService) GetBucketKeys() error {
	if !f.Connectivity.IsOnline() {
		return f.logger.Error("you are offline")
	}

	resp, err := f.Request(types.Request{
		Url:    "wf/backblaze-upload-access",
		Method: "GET",
		Body:   map[string]any{},
	})
	if err != nil {
		return err
	}

	keyID, ok := resp["key_id"].(string)
	if !ok || strings.TrimSpace(keyID) == "" {
		return f.logger.Errorf("backblaze credential response missing key_id")
	}

	applicationKey, ok := resp["application_key"].(string)
	if !ok || strings.TrimSpace(applicationKey) == "" {
		return f.logger.Errorf("backblaze credential response missing application_key")
	}

	if err := keyring.Set("credentials", "APP_ID", keyID); err != nil {
		return fmt.Errorf("store backblaze app id: %w", err)
	}
	if err := keyring.Set("credentials", "APP_KEY", applicationKey); err != nil {
		return fmt.Errorf("store backblaze app key: %w", err)
	}

	return nil
}

// ========================== UTILITY ===============================

func (f *FesClientService) OpenFesWebApp(employeeEmail string) error {
	if !f.Connectivity.IsOnline() {
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

		profileDirectory, useIncognito := fesWebAppChromeLaunchTarget(
			employeeEmail,
			resolveChromeProfileDirectory,
		)
		if useIncognito {
			f.logger.Infof("op=open_fes_web_app_browser mode=incognito")
			return f.openIncognitoBrowserUrl(url)
		}

		f.logger.Infof("op=open_fes_web_app_browser mode=profile")
		if err := openChromeProfile(profileDirectory, url); err != nil {
			return f.logger.Errorf("open First Encoding web app in Chrome profile: %w", err)
		}

	} else {
		return err
	}

	return nil
}

// fesWebAppChromeLaunchTarget selects the employee's signed-in Chrome profile
// when it is available locally. Profile discovery is intentionally best-effort:
// users without a matching profile continue in a fresh incognito window.
func fesWebAppChromeLaunchTarget(employeeEmail string, resolveProfile func(string) (string, error)) (string, bool) {
	if strings.TrimSpace(employeeEmail) == "" {
		return "", true
	}

	profileDirectory, err := resolveProfile(employeeEmail)
	if err != nil || strings.TrimSpace(profileDirectory) == "" {
		return "", true
	}

	return profileDirectory, false
}

func (f *FesClientService) FetchClientAccounts() ([]ClientAccount, error) {
	if !f.Connectivity.IsOnline() {
		return nil, f.logger.Error("you are offline")
	}

	records, err := f.fetchClientAccountRecords()
	if err != nil {
		return nil, err
	}

	accounts := make([]ClientAccount, 0, len(records))
	for _, record := range records {
		display := strings.TrimSpace(record.Display)
		if display == "" {
			continue
		}

		accounts = append(accounts, ClientAccount{
			ID:               record.ID,
			Display:          display,
			Platform:         strings.TrimSpace(record.Platform),
			Holder:           strings.TrimSpace(record.Holder),
			HasCredentials:   record.Username != "" && record.Password != "",
			GoogleProfile:    strings.TrimSpace(record.GoogleProfile),
			HasGoogleProfile: strings.TrimSpace(record.GoogleProfile) != "",
		})
	}

	return accounts, nil
}

func (f *FesClientService) OpenClientAccount(ctx context.Context, accountID string) error {
	if !f.Connectivity.IsOnline() {
		return f.logger.Error("you are offline")
	}

	trimmedID := strings.TrimSpace(accountID)
	if trimmedID == "" {
		return f.logger.Error("client account id is required")
	}

	records, err := f.fetchClientAccountRecords()
	if err != nil {
		return err
	}

	var selected *clientAccountRecord
	for i := range records {
		if strings.TrimSpace(records[i].ID) == trimmedID {
			selected = &records[i]
			break
		}
	}

	if selected == nil {
		return f.logger.Errorf("client account not found")
	}

	if !strings.EqualFold(strings.TrimSpace(selected.Platform), "Upwork") {
		return f.logger.Errorf("only Upwork accounts are supported")
	}

	return f.launchUpworkAccountRecord(selected)
}

// ========================== CLIENTS & PROJECTS ===============================
func (f *FesClientService) SyncClientsAndProjects() error {
	var syncErrs []error

	if err := f.fetchClients(); err != nil {
		syncErrs = append(syncErrs, fmt.Errorf("clients sync failed: %w", err))
	}

	if err := f.fetchProjects(); err != nil {
		syncErrs = append(syncErrs, fmt.Errorf("projects sync failed: %w", err))
	}

	return errors.Join(syncErrs...)
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

			if strings.TrimSpace(id) == "" {
				f.logger.Printf("skipping client row with empty id")
				continue
			}

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

			if strings.TrimSpace(id) == "" {
				f.logger.Printf("skipping task row with empty id")
				continue
			}

			var clientID *string
			trimmedClientID := strings.TrimSpace(clientId)
			if trimmedClientID != "" {
				exists, err := f.TaskRepository.ClientExists(trimmedClientID)
				if err != nil {
					return fmt.Errorf("validate client %s for task %s: %w", trimmedClientID, id, err)
				}
				if exists {
					clientID = db.StringPtr(trimmedClientID)
				} else {
					f.logger.Printf("missing local client %s for task %s; storing task without client link", trimmedClientID, id)
				}
			}

			tasks = append(tasks, db.Task{
				TaskID:      id,
				Title:       account,
				ClientID:    clientID,
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

func (f *FesClientService) FetchUpworkCreds(ctx context.Context, taskID string) error {
	if !f.Connectivity.IsOnline() {
		return f.logger.Error("you are offline")
	}

	task, err := f.TaskRepository.GetTaskFromTaskID(taskID)
	if err != nil {
		log.Printf("error on getting task: %v", err)
		return err
	}

	creds, err := f.fetchUpworkCredentialsByAccount(task.Title)
	if err != nil {
		return err
	}

	f.logCredentialResolution(task.Title, "workflow", creds)

	if err := f.openIncognitoBrowserUrl(upworkChromeLoginURL); err != nil {
		return err
	}

	return f.launchUpworkAutomation(ctx, task.Title, creds)
}

func (f *FesClientService) fetchClientAccountRecords() ([]clientAccountRecord, error) {
	if !f.Connectivity.IsOnline() {
		return nil, f.logger.Error("you are offline")
	}

	response, err := f.Request(types.Request{
		Url:    "wf/handled_projects",
		Method: "POST",
		Body:   map[string]any{},
	})
	if err != nil {
		return nil, err
	}

	records, found, err := parseClientAccountRecords(response)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, f.logger.Errorf("handled_projects response is missing client_lists")
	}

	return records, nil
}

func parseClientAccountRecords(response map[string]any) ([]clientAccountRecord, bool, error) {
	rawRecords, exists := response["client_lists"]
	if !exists {
		return nil, false, nil
	}

	list, ok := rawRecords.([]any)
	if !ok {
		return nil, true, fmt.Errorf("client_lists has invalid format")
	}

	records := make([]clientAccountRecord, 0, len(list))
	for _, item := range list {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}

		records = append(records, clientAccountRecord{
			ID:            normalizeCredentialValue(record["_id"]),
			Display:       normalizeCredentialValue(record["Display"]),
			Platform:      normalizeCredentialValue(record["Platform"]),
			Holder:        normalizeCredentialValue(record["Holder"]),
			Username:      normalizeCredentialValue(record["credentials - email"]),
			Password:      normalizeCredentialValue(record["credentials - password"]),
			GoogleProfile: normalizeCredentialValue(record["Google profile"]),
		})
	}

	return records, true, nil
}

func (f *FesClientService) fetchUpworkCredentialsByAccount(accountLabel string) (*upworkCredentials, error) {
	trimmedAccountLabel := strings.TrimSpace(accountLabel)
	if trimmedAccountLabel == "" {
		return nil, f.logger.Error("account name is required")
	}

	response, err := f.Request(types.Request{
		Method: "POST",
		Url:    "wf/fetch-credentials/",
		Body: map[string]any{
			"account": trimmedAccountLabel,
		},
	})
	if err != nil {
		return nil, err
	}

	creds := parseUpworkCredentialsResponse(response)
	if creds.Username == "" || creds.Password == "" {
		return nil, f.logger.Errorf("client account credentials are incomplete")
	}

	return creds, nil
}

func (f *FesClientService) resolveUpworkCredentials(record *clientAccountRecord) (*upworkCredentials, string, error) {
	if record == nil {
		return nil, "", f.logger.Error("client account record is required")
	}

	if workflowCreds, err := f.fetchUpworkCredentialsByAccount(record.Display); err == nil {
		return workflowCreds, "workflow", nil
	} else if directCreds := directUpworkCredentialsFromRecord(record); directCreds != nil {
		f.logger.Warnf(
			"op=upwork_credentials_fallback account=%q workflow_err=%v direct_username=%s direct_password=%s",
			strings.TrimSpace(record.Display),
			err,
			describeCredentialValue(directCreds.Username),
			describeCredentialValue(directCreds.Password),
		)
		return directCreds, "handled_projects", nil
	} else {
		return nil, "", err
	}
}

func (f *FesClientService) launchUpworkAccountRecord(selected *clientAccountRecord) error {
	if selected == nil {
		return f.logger.Errorf("client account record is required")
	}

	creds, source, err := f.resolveUpworkCredentials(selected)
	if err != nil {
		return err
	}

	if strings.TrimSpace(selected.GoogleProfile) == "" {
		return f.logger.Errorf("google chrome profile email is required for this client account")
	}

	profileDirectory, err := resolveChromeProfileDirectory(selected.GoogleProfile)
	if err != nil {
		return err
	}

	f.logger.Infof(
		"op=client_account_chrome_profile_selected account=%q google_profile=%q profile_directory=%q",
		strings.TrimSpace(selected.Display),
		strings.TrimSpace(selected.GoogleProfile),
		profileDirectory,
	)

	f.logCredentialResolution(selected.Display, source, creds)

	if err := openChromeProfile(profileDirectory, upworkChromeLoginURL); err != nil {
		return f.logger.Errorf("failed to open Chrome profile for %q: %w", selected.Display, err)
	}

	return f.launchClientAccountUpworkAutomation(selected.Display, creds)
}

func (f *FesClientService) launchUpworkAutomation(ctx context.Context, accountLabel string, creds *upworkCredentials) error {
	fmt.Print("Fetching Upwork Credentials Please wait")
	return f.launchUpworkLoginAutomation(ctx, accountLabel, creds, true)
}

func (f *FesClientService) launchClientAccountUpworkAutomation(accountLabel string, creds *upworkCredentials) error {
	return f.launchUpworkLoginAutomation(nil, accountLabel, creds, false)
}

func upworkLoginFormPasteValues(creds *upworkCredentials) (string, string, error) {
	if creds == nil {
		return "", "", fmt.Errorf("missing Upwork credentials")
	}

	username := strings.TrimSpace(creds.Username)
	password := strings.TrimSpace(creds.Password)
	if username == "" || password == "" {
		return "", "", fmt.Errorf("client account credentials are incomplete")
	}

	return username, password, nil
}

func normalizeCredentialValue(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(raw)
}

func parseUpworkCredentialsResponse(response map[string]any) *upworkCredentials {
	return &upworkCredentials{
		Username: normalizeCredentialValue(response["username"]),
		Password: normalizeCredentialValue(response["password"]),
	}
}

func directUpworkCredentialsFromRecord(record *clientAccountRecord) *upworkCredentials {
	if record == nil {
		return nil
	}

	creds := &upworkCredentials{
		Username: strings.TrimSpace(record.Username),
		Password: strings.TrimSpace(record.Password),
	}
	if creds.Username == "" || creds.Password == "" {
		return nil
	}

	return creds
}

func describeCredentialValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "empty"
	}

	return fmt.Sprintf("len=%d has_at=%t", len(trimmed), strings.Contains(trimmed, "@"))
}

func (f *FesClientService) logCredentialResolution(accountLabel, source string, creds *upworkCredentials) {
	if f == nil || f.logger == nil || creds == nil {
		return
	}

	f.logger.Infof(
		"op=upwork_credentials_selected account=%q source=%s username=%s password=%s",
		strings.TrimSpace(accountLabel),
		strings.TrimSpace(source),
		describeCredentialValue(creds.Username),
		describeCredentialValue(creds.Password),
	)
}

func pasteClipboardValue(kb *keybd_event.KeyBonding, value string) error {
	if kb == nil {
		return fmt.Errorf("keyboard binding is required")
	}

	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		clipboard.WriteAll("")
		return fmt.Errorf("error copy")
	}

	clipboard.WriteAll(trimmedValue)
	time.Sleep(150 * time.Millisecond)

	kb.Clear()
	kb.SetKeys(keybd_event.VK_V)
	kb.HasCTRL(true)
	return kb.Launching()
}

func pressKey(kb *keybd_event.KeyBonding, key int) error {
	if kb == nil {
		return fmt.Errorf("keyboard binding is required")
	}

	kb.Clear()
	kb.SetKeys(key)
	return kb.Launching()
}

func clearClipboard() {
	time.Sleep(upworkClipboardClearDelay)
	clipboard.WriteAll("")
}

func (f *FesClientService) launchUpworkLoginAutomation(ctx context.Context, accountLabel string, creds *upworkCredentials, emitCompletionEvent bool) error {
	firstPasteValue, secondPasteValue, err := upworkLoginFormPasteValues(creds)
	if err != nil {
		return f.logger.Error(err.Error())
	}

	f.logger.Printf("Opening Upwork login form for account: %s", strings.TrimSpace(accountLabel))

	go func() {
		defer clearClipboard()
		if emitCompletionEvent && ctx != nil {
			defer runtime.EventsEmit(ctx, "upwork-button", true)
		}

		if err := f.performUpworkLoginAutomation(firstPasteValue, secondPasteValue); err != nil {
			f.logger.Warnf("op=upwork_login_automation_failed account=%q err=%v", strings.TrimSpace(accountLabel), err)
		}
	}()

	return nil
}

func (f *FesClientService) performUpworkLoginAutomation(username, password string) error {
	kb, _ := keybd_event.NewKeyBonding()
	if _, err := f.waitForUpworkLoginTarget(upworkDevToolsTargetTimeout); err != nil {
		f.logger.Warnf("op=upwork_login_ready_check_unavailable err=%v; falling back to legacy timing", err)
		return performLegacyUpworkLogin(&kb, username, password)
	}
	usernameSelector, err := f.waitForUpworkFieldFocus("username", upworkUsernameFieldSelectors, upworkFieldReadyTimeout)
	if err != nil {
		return err
	}
	f.logger.Infof("op=upwork_username_field_ready selector=%q", usernameSelector)
	if err := pasteClipboardValue(&kb, username); err != nil {
		return err
	}
	time.Sleep(upworkFieldTransitionDelay)
	if err := pressKey(&kb, keybd_event.VK_ENTER); err != nil {
		return err
	}
	passwordSelector, err := f.waitForUpworkFieldFocus("password", upworkPasswordFieldSelectors, upworkFieldReadyTimeout)
	if err != nil {
		return err
	}
	f.logger.Infof("op=upwork_password_field_ready selector=%q", passwordSelector)
	if err := pasteClipboardValue(&kb, password); err != nil {
		return err
	}
	time.Sleep(upworkClientAccountSubmitDelay)
	return pressKey(&kb, keybd_event.VK_ENTER)
}

func performLegacyUpworkLogin(kb *keybd_event.KeyBonding, firstPasteValue, secondPasteValue string) error {
	if kb == nil {
		return fmt.Errorf("keyboard binding is required")
	}

	time.Sleep(upworkClientAccountInitialFocusDelay)
	if err := pasteClipboardValue(kb, firstPasteValue); err != nil {
		return err
	}

	time.Sleep(upworkFieldTransitionDelay)
	if err := pressKey(kb, keybd_event.VK_TAB); err != nil {
		return err
	}

	time.Sleep(upworkFieldTransitionDelay)
	if err := pressKey(kb, keybd_event.VK_ENTER); err != nil {
		return err
	}

	time.Sleep(upworkClientAccountPasswordDelay)
	if err := pasteClipboardValue(kb, secondPasteValue); err != nil {
		return err
	}

	time.Sleep(upworkClientAccountSubmitDelay)
	return pressKey(kb, keybd_event.VK_ENTER)
}

func (f *FesClientService) waitForUpworkLoginTarget(timeout time.Duration) (*chromeDevToolsTarget, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		targets, err := fetchChromeDevToolsTargets()
		if err != nil {
			lastErr = err
			time.Sleep(upworkFieldReadyPollInterval)
			continue
		}

		if target := selectUpworkLoginTarget(targets); target != nil {
			return target, nil
		}

		lastErr = fmt.Errorf("Upwork login tab not found in Chrome targets")
		time.Sleep(upworkFieldReadyPollInterval)
	}

	if lastErr != nil {
		return nil, fmt.Errorf("timed out waiting for Upwork browser target: %w", lastErr)
	}

	return nil, fmt.Errorf("timed out waiting for Upwork browser target")
}

func (f *FesClientService) waitForUpworkFieldFocus(stage string, selectors []string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		target, err := f.waitForUpworkLoginTarget(upworkFieldReadyPollInterval)
		if err != nil {
			lastErr = err
			time.Sleep(upworkFieldReadyPollInterval)
			continue
		}

		selector, err := focusVisibleUpworkField(target, selectors)
		if err == nil && strings.TrimSpace(selector) != "" {
			return selector, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("%s field is not interactable yet", stage)
		}

		time.Sleep(upworkFieldReadyPollInterval)
	}

	if lastErr != nil {
		return "", fmt.Errorf("timed out waiting for Upwork %s field: %w", stage, lastErr)
	}

	return "", fmt.Errorf("timed out waiting for Upwork %s field", stage)
}

func fetchChromeDevToolsTargets() ([]chromeDevToolsTarget, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(chromeDevToolsListURL())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chrome devtools endpoint returned status %d", resp.StatusCode)
	}

	var targets []chromeDevToolsTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, fmt.Errorf("decode chrome devtools targets: %w", err)
	}

	return targets, nil
}

func selectUpworkLoginTarget(targets []chromeDevToolsTarget) *chromeDevToolsTarget {
	for i := range targets {
		target := &targets[i]
		if target.Type != "page" || strings.TrimSpace(target.WebSocketDebuggerURL) == "" {
			continue
		}
		if isUpworkLoginPageURL(target.URL) {
			return target
		}
	}

	for i := range targets {
		target := &targets[i]
		if target.Type != "page" || strings.TrimSpace(target.WebSocketDebuggerURL) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(target.URL)), "upwork.com") {
			return target
		}
	}

	return nil
}

func isUpworkLoginPageURL(rawURL string) bool {
	trimmedURL := strings.ToLower(strings.TrimSpace(rawURL))
	if trimmedURL == "" || !strings.Contains(trimmedURL, "upwork.com") {
		return false
	}

	return strings.Contains(trimmedURL, "/account-security/login") ||
		strings.Contains(trimmedURL, "/nx/login") ||
		strings.Contains(trimmedURL, "/login")
}

func focusVisibleUpworkField(target *chromeDevToolsTarget, selectors []string) (string, error) {
	if target == nil {
		return "", fmt.Errorf("chrome devtools target is required")
	}

	client, err := newChromeDevToolsClient(strings.TrimSpace(target.WebSocketDebuggerURL))
	if err != nil {
		return "", err
	}
	defer client.Close()

	if err := client.bringToFront(); err != nil {
		return "", err
	}
	if err := client.enableRuntime(); err != nil {
		return "", err
	}

	return client.focusVisibleField(selectors)
}

func newChromeDevToolsClient(wsURL string) (*chromeDevToolsClient, error) {
	trimmedURL := strings.TrimSpace(wsURL)
	if trimmedURL == "" {
		return nil, fmt.Errorf("chrome devtools websocket URL is required")
	}

	conn, _, err := websocket.DefaultDialer.Dial(trimmedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("connect chrome devtools websocket: %w", err)
	}

	return &chromeDevToolsClient{conn: conn}, nil
}

func (c *chromeDevToolsClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *chromeDevToolsClient) bringToFront() error {
	_, err := c.call("Page.bringToFront", nil)
	return err
}

func (c *chromeDevToolsClient) enableRuntime() error {
	_, err := c.call("Runtime.enable", nil)
	return err
}

func (c *chromeDevToolsClient) focusVisibleField(selectors []string) (string, error) {
	expression, err := buildUpworkFieldFocusExpression(selectors)
	if err != nil {
		return "", err
	}

	value, err := c.evaluateValue(expression)
	if err != nil {
		return "", err
	}

	selector, _ := value.(string)
	return strings.TrimSpace(selector), nil
}

func buildUpworkFieldFocusExpression(selectors []string) (string, error) {
	serializedSelectors, err := json.Marshal(selectors)
	if err != nil {
		return "", fmt.Errorf("marshal field selectors: %w", err)
	}

	return fmt.Sprintf(`(() => {
		const selectors = %s;
		const activateField = (field) => {
			try {
				field.scrollIntoView({ block: "center", inline: "center", behavior: "instant" });
			} catch (error) {
				try {
					field.scrollIntoView({ block: "center", inline: "center" });
				} catch (innerError) {}
			}

			const rect = field.getBoundingClientRect();
			const clientX = rect.left + (rect.width / 2);
			const clientY = rect.top + (rect.height / 2);
			const mouseEvents = ["pointerdown", "mousedown", "mouseup", "click"];
			for (const eventName of mouseEvents) {
				try {
					field.dispatchEvent(new MouseEvent(eventName, {
						bubbles: true,
						cancelable: true,
						view: window,
						button: 0,
						buttons: 1,
						clientX,
						clientY
					}));
				} catch (error) {}
			}

			try {
				field.click();
			} catch (error) {}

			try {
				field.focus({ preventScroll: true });
			} catch (error) {
				try {
					field.focus();
				} catch (innerError) {}
			}

			if (typeof field.select === "function") {
				try {
					field.select();
				} catch (error) {}
			}
			if (typeof field.setSelectionRange === "function") {
				try {
					const valueLength = typeof field.value === "string" ? field.value.length : 0;
					field.setSelectionRange(0, valueLength);
				} catch (error) {}
			}

			return document.activeElement === field || field.matches(":focus");
		};

		for (const selector of selectors) {
			const field = document.querySelector(selector);
			if (!(field instanceof HTMLElement)) {
				continue;
			}

			const rect = field.getBoundingClientRect();
			const style = window.getComputedStyle(field);
			const visible = !!style && style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
			const interactable = visible && !field.hasAttribute("disabled") && !field.hasAttribute("readonly") && field.getAttribute("aria-hidden") !== "true";
			if (!interactable) {
				continue;
			}

			if (activateField(field)) {
				return selector;
			}
		}

		return "";
	})()`, string(serializedSelectors)), nil
}

func (c *chromeDevToolsClient) evaluateValue(expression string) (any, error) {
	result, err := c.call("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}

	var parsed chromeRuntimeEvaluateResult
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("decode runtime evaluate result: %w", err)
	}
	if parsed.ExceptionDetails != nil {
		return nil, fmt.Errorf("chrome runtime evaluate exception: %s", strings.TrimSpace(parsed.ExceptionDetails.Text))
	}

	return parsed.Result.Value, nil
}

func (c *chromeDevToolsClient) call(method string, params map[string]any) (json.RawMessage, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("chrome devtools connection is required")
	}

	c.nextID++
	requestID := c.nextID
	if err := c.conn.WriteJSON(chromeDevToolsMessage{
		ID:     requestID,
		Method: method,
		Params: params,
	}); err != nil {
		return nil, fmt.Errorf("write chrome devtools command %s: %w", method, err)
	}

	for {
		var response chromeDevToolsMessage
		if err := c.conn.ReadJSON(&response); err != nil {
			return nil, fmt.Errorf("read chrome devtools command %s: %w", method, err)
		}
		if response.ID != requestID {
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("chrome devtools command %s failed: %s", method, strings.TrimSpace(response.Error.Message))
		}
		return response.Result, nil
	}
}

func chromeDevToolsListURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/json/list", upworkChromeRemoteDebuggingPort)
}

func chromeRemoteDebuggingFlag() string {
	return fmt.Sprintf("--remote-debugging-port=%d", upworkChromeRemoteDebuggingPort)
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
	chromeBinary, err := chromeExecutablePath()
	if err != nil {
		return err
	}

	args := []string{"--incognito", "--new-window", chromeRemoteDebuggingFlag()}
	if trimmedURL := strings.TrimSpace(url); trimmedURL != "" {
		args = append(args, trimmedURL)
	}

	cmd := exec.Command(chromeBinary, args...)
	if err := cmd.Start(); err != nil {
		return f.logger.Errorf("Error opening Chrome: %s", err.Error())
	}

	return nil
}

func resolveChromeProfileDirectory(googleProfile string) (string, error) {
	trimmedProfile := strings.TrimSpace(googleProfile)
	if trimmedProfile == "" {
		return "", fmt.Errorf("google chrome profile email is required")
	}

	localStatePath, err := chromeLocalStatePath()
	if err != nil {
		return "", err
	}

	rawLocalState, err := os.ReadFile(localStatePath)
	if err != nil {
		return "", fmt.Errorf("read chrome local state: %w", err)
	}

	return resolveChromeProfileDirectoryFromLocalState(rawLocalState, trimmedProfile)
}

func resolveChromeProfileDirectoryFromLocalState(rawLocalState []byte, googleProfile string) (string, error) {
	trimmedProfile := strings.TrimSpace(googleProfile)
	if trimmedProfile == "" {
		return "", fmt.Errorf("google chrome profile email is required")
	}

	var localState chromeLocalState
	if err := json.Unmarshal(rawLocalState, &localState); err != nil {
		return "", fmt.Errorf("parse chrome local state: %w", err)
	}

	if len(localState.Profile.InfoCache) == 0 {
		return "", fmt.Errorf("chrome local state did not include any profiles")
	}

	for profileDirectory, profile := range localState.Profile.InfoCache {
		if strings.EqualFold(strings.TrimSpace(profile.UserName), trimmedProfile) {
			return profileDirectory, nil
		}
	}

	for profileDirectory, profile := range localState.Profile.InfoCache {
		if strings.EqualFold(strings.TrimSpace(profile.Name), trimmedProfile) ||
			strings.EqualFold(strings.TrimSpace(profile.ShortcutName), trimmedProfile) ||
			strings.EqualFold(strings.TrimSpace(profile.GaiaName), trimmedProfile) {
			return profileDirectory, nil
		}
	}

	return "", fmt.Errorf("no local Google Chrome profile matched %q", trimmedProfile)
}

func chromeLocalStatePath() (string, error) {
	switch native_runtime.GOOS {
	case "windows":
		localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA is not set")
		}
		return filepath.Join(localAppData, "Google", "Chrome", "User Data", "Local State"), nil
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Local State"), nil
	case "linux":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(homeDir, ".config", "google-chrome", "Local State"), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", native_runtime.GOOS)
	}
}

func openChromeProfile(profileDirectory, targetURL string) error {
	trimmedProfileDirectory := strings.TrimSpace(profileDirectory)
	if trimmedProfileDirectory == "" {
		return fmt.Errorf("chrome profile directory is required")
	}

	chromeBinary, err := chromeExecutablePath()
	if err != nil {
		return err
	}

	args := []string{
		"--profile-directory=" + trimmedProfileDirectory,
		"--new-window",
		chromeRemoteDebuggingFlag(),
	}
	if trimmedTargetURL := strings.TrimSpace(targetURL); trimmedTargetURL != "" {
		args = append(args, trimmedTargetURL)
	}

	cmd := exec.Command(chromeBinary, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open chrome: %w", err)
	}

	return nil
}

func chromeExecutablePath() (string, error) {
	switch native_runtime.GOOS {
	case "windows":
		candidates := []string{
			filepath.Join(strings.TrimSpace(os.Getenv("LOCALAPPDATA")), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(strings.TrimSpace(os.Getenv("PROGRAMFILES")), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(strings.TrimSpace(os.Getenv("PROGRAMFILES(X86)")), "Google", "Chrome", "Application", "chrome.exe"),
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		if lookPath, err := exec.LookPath("chrome.exe"); err == nil {
			return lookPath, nil
		}
		if lookPath, err := exec.LookPath("chrome"); err == nil {
			return lookPath, nil
		}
	case "darwin":
		candidate := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	case "linux":
		for _, candidate := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
			if lookPath, err := exec.LookPath(candidate); err == nil {
				return lookPath, nil
			}
		}
	default:
		return "", fmt.Errorf("unsupported OS: %s", native_runtime.GOOS)
	}

	return "", fmt.Errorf("Google Chrome executable was not found")
}
