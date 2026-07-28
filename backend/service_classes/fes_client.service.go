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
	"fesapp/internal/buildinfo"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"os/exec"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/gorilla/websocket"
	"github.com/lxn/win"
	uia "github.com/sjuhan/w32uiautomation"
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

type upworkReadyField struct {
	Selector string
	X        float64
	Y        float64
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
	productionFESAPIBaseURL  = "https://firstencoding.app/api/1.1/"
	developmentFESAPIBaseURL = "https://firstencoding.app/version-test/api/1.1/"

	upworkClientAccountInitialFocusDelay = 4 * time.Second
	upworkClientAccountPasswordDelay     = 2500 * time.Millisecond
	upworkClientAccountSubmitDelay       = 500 * time.Millisecond
	upworkClientControlRetryLogInterval  = 5 * time.Second
	upworkFieldTransitionDelay           = 350 * time.Millisecond
	upworkFieldReadyTimeout              = 20 * time.Second
	upworkDevToolsTargetTimeout          = 8 * time.Second
	upworkFieldReadyPollInterval         = 250 * time.Millisecond
	upworkChromeRemoteDebuggingPort      = 9333
	upworkChromeLoginURL                 = "https://www.upwork.com/ab/account-security/login?redir=%2Fab%2Faccount-security%2Foauth2%2Fauthorize%3Fresponse_type%3Dcode%26redirect_uri%3Dupwork%253A%252F%252Fwww.upwork.com%252Foauth2redir%26client_id%3D522767a70366bdd00e95cd6e8756940c"
	upworkChromeUserDataDirPrefix        = "fes-upwork-chrome-"
	upworkChromeProfileCleanupDelay      = time.Second
	upworkChromeProfileCleanupTries      = 5
	upworkFieldPositionTolerance         = 2.0
)

var (
	user32DLL       = windows.NewLazySystemDLL("user32.dll")
	enumWindowsProc = user32DLL.NewProc("EnumWindows")

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

	_, localDevelopmentOverride := os.LookupEnv("DEVELOPMENT")
	baseUrl := fesAPIBaseURL(buildinfo.Current().Environment, localDevelopmentOverride)

	return &FesClientService{
		baseUrl:        baseUrl,
		client:         &http.Client{Timeout: 30 * time.Second},
		logger:         utils.NewLoggerService(),
		TaskRepository: repository.NewTaskRepository(),
		Connectivity:   connectivityState,
	}
}

// fesAPIBaseURL keeps development installers on Bubble's version-test app.
// DEVELOPMENT remains a local-development override for wails dev sessions.
func fesAPIBaseURL(environment string, localDevelopmentOverride bool) string {
	if localDevelopmentOverride || strings.EqualFold(strings.TrimSpace(environment), "development") {
		return developmentFESAPIBaseURL
	}

	return productionFESAPIBaseURL
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

	return f.launchUpworkAccountRecord(ctx, selected)
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

	if err := f.openIsolatedUpworkBrowserURL(upworkChromeLoginURL); err != nil {
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

func (f *FesClientService) launchUpworkAccountRecord(ctx context.Context, selected *clientAccountRecord) error {
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

	if err := openChromeProfileForUpworkClientAccount(profileDirectory, upworkChromeLoginURL); err != nil {
		return f.logger.Errorf("failed to open Chrome profile for %q: %w", selected.Display, err)
	}

	return f.launchClientAccountUpworkNativeAutomation(ctx, selected.Display, creds)
}

func (f *FesClientService) launchClientAccountUpworkNativeAutomation(ctx context.Context, accountLabel string, creds *upworkCredentials) error {
	username, password, err := upworkLoginFormValues(creds)
	if err != nil {
		return f.logger.Error(err.Error())
	}

	f.logger.Printf("Opening Upwork login form for account: %s", strings.TrimSpace(accountLabel))
	go func() {
		if err := performLegacyClientAccountUpworkLogin(ctx, username, password, func(stage string, err error) {
			f.logger.Infof("op=client_account_upwork_native_automation_waiting account=%q stage=%q err=%v", strings.TrimSpace(accountLabel), stage, err)
		}); err != nil {
			f.logger.Warnf("op=client_account_upwork_native_automation_failed account=%q err=%v", strings.TrimSpace(accountLabel), err)
			return
		}
		f.logger.Infof("op=client_account_upwork_native_automation_complete account=%q method=windows_send_input", strings.TrimSpace(accountLabel))
	}()

	return nil
}

func (f *FesClientService) launchUpworkAutomation(ctx context.Context, accountLabel string, creds *upworkCredentials) error {
	fmt.Print("Fetching Upwork Credentials Please wait")
	return f.launchUpworkLoginAutomation(ctx, accountLabel, creds, true)
}

func upworkLoginFormValues(creds *upworkCredentials) (string, string, error) {
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

type upworkClientControlAction func(*uia.IUIAutomationElement) error
type upworkClientControlRetry func(stage string, err error)

func performLegacyClientAccountUpworkLogin(ctx context.Context, username, password string, onRetry upworkClientControlRetry) error {
	if err := waitForUpworkClientDelay(ctx, upworkClientAccountInitialFocusDelay); err != nil {
		return err
	}
	if err := waitForUpworkClientControl(ctx, "username input", uia.UIA_EditControlTypeId, []string{"Email or username", "Email or Username", "Username or email", "Username or Email", "Email", "Username"}, func(element *uia.IUIAutomationElement) error {
		if err := element.SetFocus(); err != nil {
			return err
		}
		return sendNativeUnicodeText(username)
	}, onRetry); err != nil {
		return fmt.Errorf("find and enter Upwork username input: %w", err)
	}

	if err := waitForUpworkClientDelay(ctx, upworkFieldTransitionDelay); err != nil {
		return err
	}
	if err := waitForUpworkClientControl(ctx, "username submit button", uia.UIA_ButtonControlTypeId, []string{"Continue", "Next", "Log in", "Log In", "Sign in", "Sign In"}, uia.Invoke, onRetry); err != nil {
		return fmt.Errorf("find and invoke Upwork username submit button: %w", err)
	}

	if err := waitForUpworkClientDelay(ctx, upworkClientAccountPasswordDelay); err != nil {
		return err
	}
	if err := waitForUpworkClientControl(ctx, "password input", uia.UIA_EditControlTypeId, []string{"Password"}, func(element *uia.IUIAutomationElement) error {
		if err := element.SetFocus(); err != nil {
			return err
		}
		return sendNativeUnicodeText(password)
	}, onRetry); err != nil {
		return fmt.Errorf("find and enter Upwork password input: %w", err)
	}

	if err := waitForUpworkClientDelay(ctx, upworkClientAccountSubmitDelay); err != nil {
		return err
	}
	if err := waitForUpworkClientControl(ctx, "password submit button", uia.UIA_ButtonControlTypeId, []string{"Log in", "Log In", "Sign in", "Sign In", "Continue"}, uia.Invoke, onRetry); err != nil {
		return fmt.Errorf("find and invoke Upwork password submit button: %w", err)
	}

	return nil
}

func waitForUpworkClientDelay(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("Upwork client account automation cancelled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func waitForUpworkClientControl(ctx context.Context, stage string, controlType int, names []string, action upworkClientControlAction, onRetry upworkClientControlRetry) error {
	if len(names) == 0 {
		return fmt.Errorf("at least one expected Upwork control name is required")
	}
	if action == nil {
		return fmt.Errorf("Upwork control action is required")
	}

	return retryUpworkClientControl(ctx, stage, upworkFieldReadyPollInterval, func() error {
		return actOnForegroundUpworkClientControl(controlType, names, action)
	}, onRetry)
}

func retryUpworkClientControl(ctx context.Context, stage string, pollInterval time.Duration, attempt func() error, onRetry upworkClientControlRetry) error {
	if attempt == nil {
		return fmt.Errorf("Upwork control attempt is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if pollInterval <= 0 {
		pollInterval = upworkFieldReadyPollInterval
	}

	var lastRetryLog time.Time
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("waiting for Upwork %s was cancelled: %w", strings.TrimSpace(stage), err)
		}
		if err := attempt(); err == nil {
			return nil
		} else if onRetry != nil && (lastRetryLog.IsZero() || time.Since(lastRetryLog) >= upworkClientControlRetryLogInterval) {
			onRetry(strings.TrimSpace(stage), err)
			lastRetryLog = time.Now()
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("waiting for Upwork %s was cancelled: %w", strings.TrimSpace(stage), ctx.Err())
		case <-timer.C:
		}
	}
}

func actOnForegroundUpworkClientControl(controlType int, names []string, action upworkClientControlAction) error {
	native_runtime.LockOSThread()
	defer native_runtime.UnlockOSThread()
	if err := ole.CoInitialize(0); err != nil {
		return err
	}
	defer ole.CoUninitialize()

	auto, err := uia.NewUIAutomation()
	if err != nil {
		return err
	}
	defer auto.Release()
	hwnd, window, err := findUpworkClientChromeWindow(auto)
	if err != nil {
		return err
	}
	defer window.Release()
	if win.GetForegroundWindow() != hwnd {
		win.ShowWindow(hwnd, win.SW_RESTORE)
		if !win.SetForegroundWindow(hwnd) || win.GetForegroundWindow() != hwnd {
			return fmt.Errorf("could not activate the Upwork Chrome window")
		}
	}
	for _, name := range names {
		condition, err := upworkClientControlCondition(auto, controlType, name)
		if err != nil {
			continue
		}
		element, err := window.FindFirst(uia.TreeScope_Subtree, condition)
		condition.Release()
		if err != nil || element == nil {
			continue
		}
		rect, rectErr := element.Get_CurrentBoundingRectangle()
		enabled, enabledErr := uiAutomationElementIsEnabled(element)
		offscreen, offscreenErr := uiAutomationElementIsOffscreen(element)
		if rectErr != nil || enabledErr != nil || offscreenErr != nil || !enabled || offscreen || rect.Right <= rect.Left || rect.Bottom <= rect.Top {
			element.Release()
			continue
		}
		actionErr := action(element)
		element.Release()
		if actionErr != nil {
			return fmt.Errorf("activate Upwork %s control %q: %w", upworkClientControlTypeName(controlType), name, actionErr)
		}
		return nil
	}
	return fmt.Errorf("expected Upwork %s control was not found", upworkClientControlTypeName(controlType))
}

func findUpworkClientChromeWindow(auto *uia.IUIAutomation) (win.HWND, *uia.IUIAutomationElement, error) {
	if auto == nil {
		return 0, nil, fmt.Errorf("Windows UI Automation is required")
	}

	for _, hwnd := range upworkClientWindowHandles() {
		window, err := uiAutomationElementFromHandle(auto, hwnd)
		if err != nil {
			continue
		}
		className, classNameErr := window.Get_CurrentCurrentClassName()
		title, titleErr := window.Get_CurrentName()
		if classNameErr == nil && titleErr == nil && className == "Chrome_WidgetWin_1" && strings.Contains(strings.ToLower(title), "upwork") {
			return hwnd, window, nil
		}
		window.Release()
	}

	return 0, nil, fmt.Errorf("no visible Upwork Chrome window was found")
}

func upworkClientWindowHandles() []win.HWND {
	handles := make([]win.HWND, 0, 8)
	seen := make(map[win.HWND]struct{})
	add := func(hwnd win.HWND) {
		if hwnd == 0 || !win.IsWindowVisible(hwnd) {
			return
		}
		if _, exists := seen[hwnd]; exists {
			return
		}
		seen[hwnd] = struct{}{}
		handles = append(handles, hwnd)
	}

	add(win.GetForegroundWindow())
	callback := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		add(win.HWND(hwnd))
		return 1
	})
	_, _, _ = enumWindowsProc.Call(callback, 0)

	return handles
}

func upworkClientControlCondition(auto *uia.IUIAutomation, controlType int, name string) (*uia.IUIAutomationCondition, error) {
	if auto == nil {
		return nil, fmt.Errorf("Windows UI Automation is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("Upwork control name is required")
	}

	nameCondition, err := auto.CreatePropertyCondition(uia.UIA_NamePropertyId, uia.NewVariantString(name))
	if err != nil {
		return nil, err
	}
	defer nameCondition.Release()

	typeCondition, err := auto.CreatePropertyCondition(uia.UIA_ControlTypePropertyId, ole.NewVariant(ole.VT_I4, int64(controlType)))
	if err != nil {
		return nil, err
	}
	defer typeCondition.Release()

	return auto.CreateAndCondition(nameCondition, typeCondition)
}

func upworkClientControlTypeName(controlType int) string {
	switch controlType {
	case uia.UIA_EditControlTypeId:
		return "input"
	case uia.UIA_ButtonControlTypeId:
		return "button"
	default:
		return "automation"
	}
}

func uiAutomationElementFromHandle(auto *uia.IUIAutomation, hwnd win.HWND) (*uia.IUIAutomationElement, error) {
	var element *uia.IUIAutomationElement
	hr, _, _ := syscall.Syscall(auto.VTable().ElementFromHandle, 3, uintptr(unsafe.Pointer(auto)), uintptr(hwnd), uintptr(unsafe.Pointer(&element)))
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return element, nil
}

func uiAutomationElementIsEnabled(element *uia.IUIAutomationElement) (bool, error) {
	if element == nil {
		return false, fmt.Errorf("Windows UI Automation element is required")
	}
	var enabled int32
	hr, _, _ := syscall.Syscall(element.VTable().Get_CurrentIsEnabled, 2, uintptr(unsafe.Pointer(element)), uintptr(unsafe.Pointer(&enabled)), 0)
	if hr != 0 {
		return false, ole.NewError(hr)
	}
	return enabled != 0, nil
}

func uiAutomationElementIsOffscreen(element *uia.IUIAutomationElement) (bool, error) {
	if element == nil {
		return false, fmt.Errorf("Windows UI Automation element is required")
	}
	var offscreen int32
	hr, _, _ := syscall.Syscall(element.VTable().Get_CurrentIsOffscreen, 2, uintptr(unsafe.Pointer(element)), uintptr(unsafe.Pointer(&offscreen)), 0)
	if hr != 0 {
		return false, ole.NewError(hr)
	}
	return offscreen != 0, nil
}

func sendNativeUnicodeText(value string) error {
	inputs, err := nativeUnicodeKeyInputs(value)
	if err != nil {
		return err
	}
	return sendNativeKeyboardInputs(inputs)
}

func sendNativeVirtualKey(key uint16) error {
	return sendNativeKeyboardInputs(nativeVirtualKeyInputs(key))
}

func nativeUnicodeKeyInputs(value string) ([]win.KEYBD_INPUT, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("text to enter is required")
	}

	encoded := utf16.Encode([]rune(value))
	inputs := make([]win.KEYBD_INPUT, 0, len(encoded)*2)
	for _, codeUnit := range encoded {
		inputs = append(inputs,
			win.KEYBD_INPUT{Type: win.INPUT_KEYBOARD, Ki: win.KEYBDINPUT{WScan: codeUnit, DwFlags: win.KEYEVENTF_UNICODE}},
			win.KEYBD_INPUT{Type: win.INPUT_KEYBOARD, Ki: win.KEYBDINPUT{WScan: codeUnit, DwFlags: win.KEYEVENTF_UNICODE | win.KEYEVENTF_KEYUP}},
		)
	}

	return inputs, nil
}

func nativeVirtualKeyInputs(key uint16) []win.KEYBD_INPUT {
	return []win.KEYBD_INPUT{
		{Type: win.INPUT_KEYBOARD, Ki: win.KEYBDINPUT{WVk: key}},
		{Type: win.INPUT_KEYBOARD, Ki: win.KEYBDINPUT{WVk: key, DwFlags: win.KEYEVENTF_KEYUP}},
	}
}

func sendNativeKeyboardInputs(inputs []win.KEYBD_INPUT) error {
	if len(inputs) == 0 {
		return fmt.Errorf("keyboard inputs are required")
	}

	sent := win.SendInput(uint32(len(inputs)), unsafe.Pointer(&inputs[0]), int32(unsafe.Sizeof(inputs[0])))
	if sent != uint32(len(inputs)) {
		return fmt.Errorf("Windows accepted %d of %d keyboard events", sent, len(inputs))
	}

	return nil
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

func (f *FesClientService) launchUpworkLoginAutomation(ctx context.Context, accountLabel string, creds *upworkCredentials, emitCompletionEvent bool) error {
	username, password, err := upworkLoginFormValues(creds)
	if err != nil {
		return f.logger.Error(err.Error())
	}

	f.logger.Printf("Opening Upwork login form for account: %s", strings.TrimSpace(accountLabel))

	go func() {
		if emitCompletionEvent && ctx != nil {
			defer runtime.EventsEmit(ctx, "upwork-button", true)
		}

		if err := f.performUpworkLoginAutomation(username, password); err != nil {
			f.logger.Warnf("op=upwork_login_automation_failed account=%q err=%v", strings.TrimSpace(accountLabel), err)
		}
	}()

	return nil
}

func (f *FesClientService) performUpworkLoginAutomation(username, password string) error {
	if _, err := f.waitForUpworkLoginTarget(upworkDevToolsTargetTimeout); err != nil {
		return fmt.Errorf("secure Upwork form entry requires Chrome DevTools access: %w", err)
	}
	usernameSelector, target, err := f.waitForUpworkFieldEntry("username", upworkUsernameFieldSelectors, username, upworkFieldReadyTimeout)
	if err != nil {
		return err
	}
	f.logger.Infof("op=upwork_username_field_ready selector=%q", usernameSelector)
	time.Sleep(upworkFieldTransitionDelay)
	if err := pressEnterInUpworkTarget(target); err != nil {
		return err
	}
	passwordSelector, target, err := f.waitForUpworkFieldEntry("password", upworkPasswordFieldSelectors, password, upworkFieldReadyTimeout)
	if err != nil {
		return err
	}
	f.logger.Infof("op=upwork_password_field_ready selector=%q", passwordSelector)
	time.Sleep(upworkClientAccountSubmitDelay)
	return pressEnterInUpworkTarget(target)
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

func (f *FesClientService) waitForUpworkFieldEntry(stage string, selectors []string, value string, timeout time.Duration) (string, *chromeDevToolsTarget, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	var previousTargetID string
	var previousField *upworkReadyField

	for time.Now().Before(deadline) {
		target, err := f.waitForUpworkLoginTarget(upworkFieldReadyPollInterval)
		if err != nil {
			lastErr = err
			previousTargetID = ""
			previousField = nil
			time.Sleep(upworkFieldReadyPollInterval)
			continue
		}

		field, err := findReadyUpworkField(target, selectors)
		if err != nil {
			lastErr = err
			previousTargetID = ""
			previousField = nil
			time.Sleep(upworkFieldReadyPollInterval)
			continue
		}
		if field == nil {
			lastErr = fmt.Errorf("%s field is not interactable yet", stage)
			previousTargetID = ""
			previousField = nil
			time.Sleep(upworkFieldReadyPollInterval)
			continue
		}

		targetID := strings.TrimSpace(target.ID) + "|" + strings.TrimSpace(target.WebSocketDebuggerURL)
		if previousField == nil || previousTargetID != targetID || !upworkFieldPositionIsStable(previousField, field) {
			previousTargetID = targetID
			previousField = field
			time.Sleep(upworkFieldReadyPollInterval)
			continue
		}

		selector, method, err := enterTextInReadyUpworkField(target, field, value)
		if err == nil && strings.TrimSpace(selector) != "" {
			if f.logger != nil {
				f.logger.Infof("op=upwork_field_value_set stage=%s selector=%q method=%s", stage, selector, method)
			}
			return selector, target, nil
		}
		lastErr = err
		previousTargetID = ""
		previousField = nil

		time.Sleep(upworkFieldReadyPollInterval)
	}

	if lastErr != nil {
		return "", nil, fmt.Errorf("timed out waiting for Upwork %s field: %w", stage, lastErr)
	}

	return "", nil, fmt.Errorf("timed out waiting for Upwork %s field", stage)
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

func findReadyUpworkField(target *chromeDevToolsTarget, selectors []string) (*upworkReadyField, error) {
	if target == nil {
		return nil, fmt.Errorf("chrome devtools target is required")
	}

	client, err := newChromeDevToolsClient(strings.TrimSpace(target.WebSocketDebuggerURL))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.enableRuntime(); err != nil {
		return nil, err
	}

	return client.readyUpworkField(selectors)
}

func enterTextInReadyUpworkField(target *chromeDevToolsTarget, field *upworkReadyField, value string) (string, string, error) {
	if target == nil {
		return "", "", fmt.Errorf("chrome devtools target is required")
	}
	if field == nil || strings.TrimSpace(field.Selector) == "" {
		return "", "", fmt.Errorf("ready Upwork field is required")
	}

	client, err := newChromeDevToolsClient(strings.TrimSpace(target.WebSocketDebuggerURL))
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	if err := client.bringToFront(); err != nil {
		return "", "", err
	}
	if err := client.enableRuntime(); err != nil {
		return "", "", err
	}

	if err := client.clickAt(field.X, field.Y); err != nil {
		return "", "", err
	}
	if err := client.verifyFocusedField(field.Selector); err != nil {
		return "", "", err
	}
	method, err := client.setFocusedFieldValue(field.Selector, value)
	if err != nil {
		return "", "", err
	}

	return field.Selector, method, nil
}

func pressEnterInUpworkTarget(target *chromeDevToolsTarget) error {
	if target == nil {
		return fmt.Errorf("chrome devtools target is required")
	}

	client, err := newChromeDevToolsClient(strings.TrimSpace(target.WebSocketDebuggerURL))
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.bringToFront(); err != nil {
		return err
	}

	return client.pressEnter()
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

func (c *chromeDevToolsClient) readyUpworkField(selectors []string) (*upworkReadyField, error) {
	expression, err := buildUpworkFieldReadinessExpression(selectors)
	if err != nil {
		return nil, err
	}

	value, err := c.evaluateValue(expression)
	if err != nil {
		return nil, err
	}

	return parseUpworkReadyField(value)
}

func (c *chromeDevToolsClient) clickAt(x, y float64) error {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x < 0 || y < 0 {
		return fmt.Errorf("valid click coordinates are required")
	}

	for _, params := range upworkMouseClickEvents(x, y) {
		if _, err := c.call("Input.dispatchMouseEvent", params); err != nil {
			return err
		}
	}

	return nil
}

func (c *chromeDevToolsClient) verifyFocusedField(selector string) error {
	trimmedSelector := strings.TrimSpace(selector)
	if trimmedSelector == "" {
		return fmt.Errorf("field selector is required")
	}

	serializedSelector, err := json.Marshal(trimmedSelector)
	if err != nil {
		return fmt.Errorf("marshal field selector: %w", err)
	}
	value, err := c.evaluateValue(fmt.Sprintf(`(() => {
		const field = document.querySelector(%s);
		return field instanceof HTMLElement && document.activeElement === field;
	})()`, string(serializedSelector)))
	if err != nil {
		return err
	}

	focused, _ := value.(bool)
	if !focused {
		return fmt.Errorf("Upwork field %q did not receive focus after click", trimmedSelector)
	}

	return nil
}

func (c *chromeDevToolsClient) insertText(value string) error {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return fmt.Errorf("text to enter is required")
	}

	_, err := c.call("Input.insertText", map[string]any{"text": trimmedValue})
	return err
}

func (c *chromeDevToolsClient) setFocusedFieldValue(selector, value string) (string, error) {
	if err := c.insertText(value); err == nil {
		matches, verifyErr := c.fieldValueMatches(selector, value)
		if verifyErr == nil && matches {
			return "chrome_input_insert_text", nil
		}
	}

	set, err := c.setNativeFocusedFieldValue(selector, value)
	if err != nil {
		return "", fmt.Errorf("set Upwork field value through native fallback: %w", err)
	}
	if !set {
		return "", fmt.Errorf("Upwork field %q did not accept the native value fallback", strings.TrimSpace(selector))
	}

	matches, err := c.fieldValueMatches(selector, value)
	if err != nil {
		return "", fmt.Errorf("verify Upwork field value after native fallback: %w", err)
	}
	if !matches {
		return "", fmt.Errorf("Upwork field %q value verification failed after native fallback", strings.TrimSpace(selector))
	}

	return "native_input_value_setter", nil
}

func (c *chromeDevToolsClient) fieldValueMatches(selector, value string) (bool, error) {
	expression, err := buildUpworkFieldValueMatchExpression(selector, value)
	if err != nil {
		return false, err
	}

	result, err := c.evaluateValue(expression)
	if err != nil {
		return false, err
	}
	matches, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("Upwork field value verification returned an invalid result")
	}

	return matches, nil
}

func (c *chromeDevToolsClient) setNativeFocusedFieldValue(selector, value string) (bool, error) {
	expression, err := buildUpworkNativeFieldValueSetExpression(selector, value)
	if err != nil {
		return false, err
	}

	result, err := c.evaluateValue(expression)
	if err != nil {
		return false, err
	}
	set, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("Upwork native value setter returned an invalid result")
	}

	return set, nil
}

func (c *chromeDevToolsClient) pressEnter() error {
	params := map[string]any{
		"key":                   "Enter",
		"code":                  "Enter",
		"windowsVirtualKeyCode": 13,
		"nativeVirtualKeyCode":  13,
		"unmodifiedText":        "\r",
		"text":                  "\r",
	}

	for _, eventType := range []string{"keyDown", "keyUp"} {
		params["type"] = eventType
		if _, err := c.call("Input.dispatchKeyEvent", params); err != nil {
			return err
		}
	}

	return nil
}

func buildUpworkFieldReadinessExpression(selectors []string) (string, error) {
	serializedSelectors, err := json.Marshal(selectors)
	if err != nil {
		return "", fmt.Errorf("marshal field selectors: %w", err)
	}

	return fmt.Sprintf(`(() => {
		const selectors = %s;
		if (document.readyState !== "complete") {
			return null;
		}

		for (const selector of selectors) {
			const field = document.querySelector(selector);
			if (!(field instanceof HTMLElement)) {
				continue;
			}

			try {
				field.scrollIntoView({ block: "center", inline: "center", behavior: "instant" });
			} catch (error) {
				try {
					field.scrollIntoView({ block: "center", inline: "center" });
				} catch (innerError) {}
			}

			const rect = field.getBoundingClientRect();
			const style = window.getComputedStyle(field);
			const visible = !!style && style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
			const interactable = visible && !field.hasAttribute("disabled") && !field.hasAttribute("readonly") && field.getAttribute("aria-hidden") !== "true";
			if (!interactable) {
				continue;
			}

			const x = rect.left + (rect.width / 2);
			const y = rect.top + (rect.height / 2);
			const elementAtClickPoint = document.elementFromPoint(x, y);
			const unobstructed = elementAtClickPoint === field || field.contains(elementAtClickPoint);
			if (unobstructed) {
				return { selector, x, y };
			}
		}

		return null;
	})()`, string(serializedSelectors)), nil
}

func buildUpworkFieldValueMatchExpression(selector, value string) (string, error) {
	serializedSelector, err := json.Marshal(strings.TrimSpace(selector))
	if err != nil {
		return "", fmt.Errorf("marshal field selector: %w", err)
	}
	serializedValue, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal field value: %w", err)
	}

	return fmt.Sprintf(`(() => {
		const field = document.querySelector(%s);
		const expectedValue = %s;
		return field instanceof HTMLInputElement && document.activeElement === field && field.value === expectedValue;
	})()`, string(serializedSelector), string(serializedValue)), nil
}

func buildUpworkNativeFieldValueSetExpression(selector, value string) (string, error) {
	serializedSelector, err := json.Marshal(strings.TrimSpace(selector))
	if err != nil {
		return "", fmt.Errorf("marshal field selector: %w", err)
	}
	serializedValue, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal field value: %w", err)
	}

	return fmt.Sprintf(`(() => {
		const field = document.querySelector(%s);
		const expectedValue = %s;
		if (!(field instanceof HTMLInputElement) || document.activeElement !== field) {
			return false;
		}

		const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
		if (typeof setter !== "function") {
			return false;
		}

		setter.call(field, expectedValue);
		field.dispatchEvent(new InputEvent("input", {
			bubbles: true,
			composed: true,
			inputType: "insertText",
			data: null
		}));
		field.dispatchEvent(new Event("change", { bubbles: true, composed: true }));
		return field.value === expectedValue;
	})()`, string(serializedSelector), string(serializedValue)), nil
}

func parseUpworkReadyField(value any) (*upworkReadyField, error) {
	if value == nil {
		return nil, nil
	}

	rawField, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid Upwork field readiness response")
	}

	selector, _ := rawField["selector"].(string)
	x, xOK := rawField["x"].(float64)
	y, yOK := rawField["y"].(float64)
	if strings.TrimSpace(selector) == "" || !xOK || !yOK || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x < 0 || y < 0 {
		return nil, fmt.Errorf("invalid Upwork field readiness response")
	}

	return &upworkReadyField{Selector: strings.TrimSpace(selector), X: x, Y: y}, nil
}

func upworkFieldPositionIsStable(previous, current *upworkReadyField) bool {
	if previous == nil || current == nil || previous.Selector != current.Selector {
		return false
	}

	return math.Abs(previous.X-current.X) <= upworkFieldPositionTolerance &&
		math.Abs(previous.Y-current.Y) <= upworkFieldPositionTolerance
}

func upworkMouseClickEvents(x, y float64) []map[string]any {
	return []map[string]any{
		{"type": "mouseMoved", "x": x, "y": y},
		{"type": "mousePressed", "x": x, "y": y, "button": "left", "clickCount": 1},
		{"type": "mouseReleased", "x": x, "y": y, "button": "left", "clickCount": 1},
	}
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

func (f *FesClientService) openIsolatedUpworkBrowserURL(url string) error {
	chromeBinary, err := chromeExecutablePath()
	if err != nil {
		return err
	}

	userDataDir, err := os.MkdirTemp("", upworkChromeUserDataDirPrefix)
	if err != nil {
		return fmt.Errorf("create isolated Upwork Chrome profile: %w", err)
	}

	args, err := upworkChromeLaunchArgs(userDataDir, url)
	if err != nil {
		_ = removeManagedUpworkChromeUserDataDir(userDataDir)
		return err
	}

	cmd := exec.Command(chromeBinary, args...)
	if err := cmd.Start(); err != nil {
		_ = removeManagedUpworkChromeUserDataDir(userDataDir)
		return f.logger.Errorf("Error opening Chrome: %s", err.Error())
	}

	go f.cleanupUpworkChromeUserDataDir(cmd, userDataDir)

	return nil
}

func upworkChromeLaunchArgs(userDataDir, url string) ([]string, error) {
	trimmedUserDataDir := strings.TrimSpace(userDataDir)
	if trimmedUserDataDir == "" {
		return nil, fmt.Errorf("isolated Upwork Chrome profile directory is required")
	}

	args := []string{
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		"--user-data-dir=" + trimmedUserDataDir,
		"--remote-debugging-address=127.0.0.1",
		chromeRemoteDebuggingFlag(),
	}
	if trimmedURL := strings.TrimSpace(url); trimmedURL != "" {
		args = append(args, trimmedURL)
	}

	return args, nil
}

func (f *FesClientService) cleanupUpworkChromeUserDataDir(cmd *exec.Cmd, userDataDir string) {
	if cmd != nil {
		if err := cmd.Wait(); err != nil && f != nil && f.logger != nil {
			f.logger.Warnf("op=upwork_chrome_process_exit err=%v", err)
		}
	}

	var cleanupErr error
	for attempt := 0; attempt < upworkChromeProfileCleanupTries; attempt++ {
		cleanupErr = removeManagedUpworkChromeUserDataDir(userDataDir)
		if cleanupErr == nil {
			return
		}
		time.Sleep(upworkChromeProfileCleanupDelay)
	}

	if f != nil && f.logger != nil {
		f.logger.Warnf("op=upwork_chrome_profile_cleanup_failed path=%q err=%v", userDataDir, cleanupErr)
	}
}

func removeManagedUpworkChromeUserDataDir(userDataDir string) error {
	cleanUserDataDir, err := filepath.Abs(strings.TrimSpace(userDataDir))
	if err != nil {
		return fmt.Errorf("resolve isolated Upwork Chrome profile directory: %w", err)
	}
	cleanTempDir, err := filepath.Abs(os.TempDir())
	if err != nil {
		return fmt.Errorf("resolve temporary directory: %w", err)
	}

	if filepath.Dir(cleanUserDataDir) != cleanTempDir || !strings.HasPrefix(filepath.Base(cleanUserDataDir), upworkChromeUserDataDirPrefix) {
		return fmt.Errorf("refusing to remove unmanaged Upwork Chrome profile directory %q", userDataDir)
	}

	return os.RemoveAll(cleanUserDataDir)
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

func openChromeProfileForUpworkClientAccount(profileDirectory, targetURL string) error {
	trimmedProfileDirectory := strings.TrimSpace(profileDirectory)
	if trimmedProfileDirectory == "" {
		return fmt.Errorf("chrome profile directory is required")
	}
	chromeBinary, err := chromeExecutablePath()
	if err != nil {
		return err
	}
	args := []string{"--profile-directory=" + trimmedProfileDirectory, "--force-renderer-accessibility", "--new-window"}
	if trimmedTargetURL := strings.TrimSpace(targetURL); trimmedTargetURL != "" {
		args = append(args, trimmedTargetURL)
	}
	cmd := exec.Command(chromeBinary, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open Chrome for Upwork client account: %w", err)
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
