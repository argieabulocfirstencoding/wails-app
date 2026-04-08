package types

type Request struct {
	Method string                 `json:"method"`
	Url    string                 `json:"url"`
	Body   map[string]interface{} `json:"body"`
}

type Response struct {
}
