package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/kayconfig/green-light-api/internal/data"
	"github.com/kayconfig/green-light-api/internal/validator"
)

// createMovieHandler godoc
// @Summary      Create a movie
// @Description  Creates a new movie record. Requires movies:write permission.
// @Tags         movies
// @Accept       json
// @Produce      json
// @Param        body  body      createMovieRequest  true  "Movie details"
// @Success      201   {object}  movieResponse
// @Failure      400   {object}  errorResponse
// @Failure      401   {object}  errorResponse
// @Failure      403   {object}  errorResponse
// @Failure      422   {object}  validationErrorResponse
// @Failure      500   {object}  errorResponse
// @Security     BearerAuth
// @Router       /v1/movies [post]
func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// initialize validator instance
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}
	v := validator.New()

	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	err = app.models.Movies.Insert(movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// when sending http respone, we include a Location header to let the
	// client know which URL they can find the newly-created resource. We make
	// an empty http.Header map and then use the Set() method to
	// add a new Location header, interpolating the system-generated
	// ID for our new movie in the URL
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))
	err = app.writeJSON(w, http.StatusCreated, envelope{
		"movie": movie,
	}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
// showMovieHandler godoc
// @Summary      Get a movie
// @Description  Retrieves a single movie by ID. Requires movies:read permission.
// @Tags         movies
// @Produce      json
// @Param        id   path      int  true  "Movie ID"
// @Success      200  {object}  movieResponse
// @Failure      401  {object}  errorResponse
// @Failure      403  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Security     BearerAuth
// @Router       /v1/movies/{id} [get]
func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}

	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Encode the struct to JSON and send it as the HTTP response.
	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// updateMovieHandler godoc
// @Summary      Update a movie
// @Description  Partially updates a movie by ID. All fields are optional; at least one must be provided. Requires movies:write permission.
// @Tags         movies
// @Accept       json
// @Produce      json
// @Param        id    path      int                 true  "Movie ID"
// @Param        body  body      updateMovieRequest  true  "Fields to update"
// @Success      200   {object}  movieResponse
// @Failure      400   {object}  errorResponse
// @Failure      401   {object}  errorResponse
// @Failure      403   {object}  errorResponse
// @Failure      404   {object}  errorResponse
// @Failure      409   {object}  errorResponse
// @Failure      422   {object}  validationErrorResponse
// @Failure      500   {object}  errorResponse
// @Security     BearerAuth
// @Router       /v1/movies/{id} [patch]
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	shouldUpdate := false

	if input.Title != nil {
		movie.Title = *input.Title
		shouldUpdate = true
	}
	if input.Year != nil {
		movie.Year = *input.Year
		shouldUpdate = true
	}

	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
		shouldUpdate = true
	}

	if input.Genres != nil {
		movie.Genres = input.Genres
		shouldUpdate = true
	}

	if !shouldUpdate {
		app.unprocessableEntityResponse(w, r,
			"provide at least one field to update")
		return
	}

	// validate the updated movie record, sending the client a 422 Unprocessable Entity
	// response if any checks fail.
	v := validator.New()

	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
			return
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteMovieHandler godoc
// @Summary      Delete a movie
// @Description  Deletes a movie by ID. Requires movies:write permission.
// @Tags         movies
// @Produce      json
// @Param        id   path      int  true  "Movie ID"
// @Success      200  {object}  messageResponse
// @Failure      401  {object}  errorResponse
// @Failure      403  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Security     BearerAuth
// @Router       /v1/movies/{id} [delete]
func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.Movies.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// listMoviesHandler godoc
// @Summary      List movies
// @Description  Returns a paginated list of movies with optional filters. Requires movies:read permission.
// @Tags         movies
// @Produce      json
// @Param        title      query     string  false  "Filter by title (case-insensitive, partial match)"
// @Param        genres     query     string  false  "Filter by genres (comma-separated, e.g. sci-fi,drama)"
// @Param        page       query     int     false  "Page number (default: 1)"
// @Param        page_size  query     int     false  "Results per page (default: 20, max: 100)"
// @Param        sort       query     string  false  "Sort field: title, year, runtime, created_at (prefix - for descending)"
// @Success      200        {object}  moviesListResponse
// @Failure      401        {object}  errorResponse
// @Failure      403        {object}  errorResponse
// @Failure      422        {object}  validationErrorResponse
// @Failure      500        {object}  errorResponse
// @Security     BearerAuth
// @Router       /v1/movies [get]
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	// expected values from the request query string
	var input struct {
		Title  string
		Genres []string
		data.Filters
	}

	v := validator.New()
	qs := r.URL.Query()

	input.Title = app.readString(qs, "title", "")
	input.Genres = app.readCSV(qs, "genres", []string{})

	input.Page = app.readInt(qs, "page", 1, v)
	input.PageSize = app.readInt(qs, "page_size", 20, v)

	input.Sort = app.readString(qs, "sort", "-created_at")
	input.SortSafeList = []string{"created_at", "title", "year", "runtime", "-created_at", "-title", "-year", "-runtime"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	movies, metadata, err := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movies": movies, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)

	}
}
