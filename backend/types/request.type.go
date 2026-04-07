package types

type Request struct {
	ReqType  string                 `json:"req_type"`
	Pathname string                 `json:"pathname"`
	Body     map[string]interface{} `json:"body"`
}

type Response struct {
}
