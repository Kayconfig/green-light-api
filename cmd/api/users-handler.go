package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/kayconfig/green-light-api/internal/data"
	"github.com/kayconfig/green-light-api/internal/validator"
)

func sendMailToUser(app *application, user *data.User, template string) {
	// a deferred function that uses recover() to catch panic.
	// then log an error message instead of terminating the application

	err := app.mailer.Send(user.Email, template, user)
	if err != nil {
		app.logger.Error(err.Error())
	}
}

// registerUser godoc
// @Summary register a user
// @Descriptioni create user account and send back authentication token
// @Tags users
// @Accept json
// @Produce json
// @Success 201
// registerUserHandler godoc
// @Summary      Register a user
// @Description  Creates a new user account and sends an activation email. The account starts inactive and must be activated before logging in.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      registerUserRequest  true  "User registration details"
// @Success      202   {object}  userResponse
// @Failure      400   {object}  errorResponse
// @Failure      422   {object}  validationErrorResponse
// @Failure      500   {object}  errorResponse
// @Router       /v1/users [post]
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.background(func() { sendMailToUser(app, user, "suspicious_login.tmpl") })
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Permissions.AddForUser(user.ID, data.PermissionsCode.MoviesRead)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	token, err := app.models.Tokens.New(
		user.ID,
		3*24*time.Hour,
		data.ScopeActivation,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		data := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}

		err := app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.Error(err.Error())
		}
	})

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// sendActivationTokenHandler godoc
// @Summary      Resend activation email
// @Description  Sends a new activation email to the given address if the account exists and is not yet activated. Always returns 200 to prevent email enumeration.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      sendActivationTokenRequest  true  "Email address"
// @Success      200   {object}  messageResponse
// @Failure      400   {object}  errorResponse
// @Failure      422   {object}  validationErrorResponse
// @Failure      500   {object}  errorResponse
// @Router       /v1/users/verification [post]
func (app *application) sendActivationTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// should protect abuse
	// we already have rate limiting
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			err = app.writeJSON(w, http.StatusOK, envelope{"message": "you will receive verification mail, if email exists"}, nil)
			if err != nil {
				app.serverErrorResponse(w, r, err)
			}
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	if !user.Activated {

		token, err := app.models.Tokens.New(
			user.ID,
			3*24*time.Hour,
			data.ScopeActivation,
		)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		app.background(func() {
			data := map[string]any{
				"activationToken": token.Plaintext,
				"userID":          user.ID,
			}

			err := app.mailer.Send(user.Email, "user_welcome.tmpl", data)
			if err != nil {
				app.logger.Error(err.Error())
			}
		})
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "you will receive verification mail, if email exists"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// activateUserHandler godoc
// @Summary      Activate a user account
// @Description  Activates a user account using the 26-character token sent to their email. The token is consumed on first use.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      activateUserRequest  true  "Activation token"
// @Success      200   {object}  userResponse
// @Failure      400   {object}  errorResponse
// @Failure      409   {object}  errorResponse
// @Failure      422   {object}  validationErrorResponse
// @Failure      500   {object}  errorResponse
// @Router       /v1/users/activated [put]
func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	user.Activated = true

	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// passwordResetHandler godoc
// @Summary      Request a password reset
// @Description  Sends a password reset email to the given address. The account must exist and be activated.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      passwordResetRequest  true  "Email address"
// @Success      202   {object}  messageResponse
// @Failure      400   {object}  errorResponse
// @Failure      403   {object}  errorResponse
// @Failure      404   {object}  errorResponse
// @Failure      422   {object}  validationErrorResponse
// @Failure      500   {object}  errorResponse
// @Router       /v1/tokens/password-reset [post]
func (app *application) passwordResetHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	if user == nil {
		app.serverErrorResponse(w, r, errors.New("passwordReset failed. user should be available"))
		return
	}
	if user.Activated != true {
		app.inactiveAccountResponse(w, r)
		return
	}

	token, err := app.models.Tokens.New(
		user.ID,
		15*time.Minute,
		data.ScopePasswordReset,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		payload := map[string]any{
			"name":               user.Name,
			"passwordResetToken": token.Plaintext,
		}

		err := app.mailer.Send(user.Email, "password_reset.tmpl", payload)
		if err != nil {
			app.logger.Error(err.Error())
		}
	})

	env := envelope{"message": "an email will be sent to you containing password reset instructions"}
	err = app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// updatePasswordHandler godoc
// @Summary      Reset password
// @Description  Updates the user's password using a valid password reset token (valid for 15 minutes).
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      updatePasswordRequest  true  "Reset token and new password"
// @Success      200   {object}  messageResponse
// @Failure      400   {object}  errorResponse
// @Failure      409   {object}  errorResponse
// @Failure      422   {object}  validationErrorResponse
// @Failure      500   {object}  errorResponse
// @Router       /v1/users/password [put]
func (app *application) updatePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PasswordResetToken string `json:"password_reset_token"`
		NewPassword        string `json:"new_password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateTokenPlaintext(v, input.PasswordResetToken)
	data.ValidatePasswordPlaintext(v, input.NewPassword)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetForToken(data.ScopePasswordReset, input.PasswordResetToken)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired password reset token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = user.Password.Set(input.NewPassword)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	err = app.models.Tokens.DeleteAllForUser(data.ScopePasswordReset, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	env := envelope{"message": "your password was reset successfully"}
	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
