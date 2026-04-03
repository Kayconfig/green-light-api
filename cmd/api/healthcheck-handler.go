package main

import (
	"net/http"
)

// healthCheckHandler godoc
// @Summary      Check API health
// @Description  Returns the current status and environment info of the API
// @Tags         healthcheck
// @Produce      json
// @Success      200  {object}  healthcheckResponse
// @Router       /v1/healthcheck [get]
func (app *application) healthCheckHandler(w http.ResponseWriter, r *http.Request) {

	err := app.writeJSON(w, http.StatusOK, envelope{
		"status": "available",
		"system_info": map[string]string{
			"environment": app.config.env,
			"version":     version,
		},
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
