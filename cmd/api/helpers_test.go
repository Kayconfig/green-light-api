package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestReadJson(t *testing.T) {
	app := application{}

	tests := []struct {
		title           string
		requestJsonBody string
		expectedBody    any
		errMsg          string
	}{
		{
			title:           "should return error for badly formed JSON",
			requestJsonBody: `{Invalid Json}`,
			expectedBody:    struct{}{},
			errMsg:          "expected error for badly formed JSON",
		},
		{
			title:           "should return error for incorrect JSON type for field",
			requestJsonBody: `{"name":10}`,
			expectedBody: new(struct {
				Name string `json:"name"`
			}),
			errMsg: "expected error for incorrect JSON field type",
		},
		{
			title:           "should return error for empty body",
			requestJsonBody: "",
			expectedBody: new(struct {
				Name string `json:"name"`
			}),
			errMsg: "expected error for empty body",
		},
		{
			title:           "should return error for unknown fields in json",
			requestJsonBody: `{"message":"Hello World"}`,
			expectedBody: new(struct {
				Name string `json:"name"`
			}),
			errMsg: "expected error for empty body",
		},
		{
			title:           "should return error when body contains more than one JSON value",
			requestJsonBody: `{"name":"Broggi"}{"name":"Janet"}`,
			expectedBody: new(struct {
				Name string `json:"name"`
			}),
			errMsg: "expected error for body containing more than one JSON value",
		},
	}

	for _, test := range tests {

		t.Run(test.title, func(t *testing.T) {
			input := test.requestJsonBody
			request, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(input))
			response := httptest.NewRecorder()
			err := app.readJSON(response, request, &test.expectedBody)
			if err == nil {
				t.Fatal(test.errMsg)
			}
		})
	}

}

func TestWriteJson(t *testing.T) {
	app := application{}
	type writeJsonInput struct {
		response *httptest.ResponseRecorder
		status   int
		data     envelope
		headers  http.Header
	}

	type jsonResponse struct {
		Message string `json:"message"`
		Data    any    `json:"data,omitempty"`
	}
	testcases := []struct {
		title  string
		input  writeJsonInput
		want   any
		errMsg string
	}{
		{
			title: "should write data to response",
			input: writeJsonInput{
				response: httptest.NewRecorder(),
				status:   http.StatusOK,
				data:     envelope{"message": "successful"},
				headers:  nil,
			},

			want: jsonResponse{Message: "successful"},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.title, func(t *testing.T) {
			response := testcase.input.response
			status := testcase.input.status
			data := testcase.input.data
			preferredHeader := testcase.input.headers

			err := app.writeJSON(response, status, data, preferredHeader)
			if err != nil {
				t.Fatalf("not expecting error for: %q", testcase.title)
			}

			contentType := response.Header().Get("Content-Type")
			if contentType != jsonContentType {
				t.Fatalf("got: %s, want: %s", contentType, jsonContentType)
			}
			var got jsonResponse
			want := testcase.want
			json.NewDecoder(response.Body).Decode(&got) // ignoring error
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("want: %v, got: %v", want, got)
			}

		})
	}
}
