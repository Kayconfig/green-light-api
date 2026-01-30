package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kayconfig/green-light-api/internal/validator"
)

type envelope map[string]any

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_567)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError
		fmt.Println(err.Error())
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case errors.As(err, &invalidUnmarshalError):
			panic(err) // when dst is not a valid pointer, this is developer error

		// if the JSON contains field which cannot be mapped to target destination
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		// check if MaxBytesError occurred - i.e the request body exceeded our size limit
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)
		default:
			return err
		}
	}

	// handle extra json fields in the body
	err = decoder.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain single JSON value")
	}
	return nil
}

func (app *application) readIDParam(r *http.Request) (int64, error) {
	idParam := chi.URLParamFromCtx(r.Context(), "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	jsonEncoding, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(jsonEncoding)
	return nil
}

// the readstring() helper returns a string value from the query string, or the provided
// default value if no matching key could not be found
func (app *application) readString(queryString url.Values, key string, defaultValue string) string {
	val := queryString.Get(key)

	if val == "" {
		return defaultValue
	}

	// otherwise return the string.
	return val
}

// The readCSV() helper reads a string value from the query string and then
// splits it into a slice on the comma character. If no matching
// key could be found, it returns the provided default value
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)
	if csv == "" {
		return defaultValue
	}

	return strings.Split(csv, ",")
}

// The readInt() helper reads a string value from the query string
// and converts it to an integer before returning. if no matching key
// could be found it returns the provided default value. If the value
// couldn't be converted to an integer, then we record an
// error message in the provided Validator instance.
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	val := qs.Get(key)

	if val == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		v.AddError(key, "must be a positive integer value")
		return defaultValue
	}

	return i
}

func (app *application) background(fn func()) {
	// Launch a background goroutine
	go func() {
		// recover any panic
		defer func() {
			if err := recover(); err != nil {
				app.logger.Error(fmt.Sprintf("%v", err))
			}
		}()

		fn()
	}()
}
