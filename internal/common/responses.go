package common

var ServerErrRespone = struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}{StatusCode: 500, Message: "unable to process request, please try again later"}
