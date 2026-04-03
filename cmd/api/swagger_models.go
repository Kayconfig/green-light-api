package main

// Request bodies

type createMovieRequest struct {
	Title   string   `json:"title" example:"Inception"`
	Year    int32    `json:"year" example:"2010"`
	Runtime string   `json:"runtime" example:"148 mins"`
	Genres  []string `json:"genres" example:"sci-fi,thriller"`
}

type updateMovieRequest struct {
	Title   *string  `json:"title,omitempty" example:"Updated Title"`
	Year    *int32   `json:"year,omitempty" example:"2011"`
	Runtime *string  `json:"runtime,omitempty" example:"150 mins"`
	Genres  []string `json:"genres,omitempty" example:"sci-fi"`
}

type registerUserRequest struct {
	Name     string `json:"name" example:"Alice Smith"`
	Email    string `json:"email" example:"alice@example.com"`
	Password string `json:"password" example:"password123"`
}

type activateUserRequest struct {
	Token string `json:"token" example:"ABCDEFGHIJKLMNOPQRSTUVWXYZ"`
}

type createAuthTokenRequest struct {
	Email    string `json:"email" example:"alice@example.com"`
	Password string `json:"password" example:"password123"`
}

type sendActivationTokenRequest struct {
	Email string `json:"email" example:"alice@example.com"`
}

type passwordResetRequest struct {
	Email string `json:"email" example:"alice@example.com"`
}

type updatePasswordRequest struct {
	PasswordResetToken string `json:"password_reset_token" example:"ABCDEFGHIJKLMNOPQRSTUVWXYZ"`
	NewPassword        string `json:"new_password" example:"newpassword123"`
}

// Response types

type movieObject struct {
	ID        int64    `json:"id" example:"1"`
	Title     string   `json:"title" example:"Inception"`
	Year      int32    `json:"year" example:"2010"`
	Runtime   string   `json:"runtime" example:"148 mins"`
	Genres    []string `json:"genres"`
	Version   int32    `json:"version" example:"1"`
	CreatedAt string   `json:"created_at" example:"2024-01-01T00:00:00Z"`
	UpdatedAt string   `json:"updated_at" example:"2024-01-01T00:00:00Z"`
}

type movieResponse struct {
	Movie movieObject `json:"movie"`
}

type metadataObject struct {
	CurrentPage  int `json:"current_page" example:"1"`
	PageSize     int `json:"page_size" example:"20"`
	FirstPage    int `json:"first_page" example:"1"`
	LastPage     int `json:"last_page" example:"5"`
	TotalRecords int `json:"total_records" example:"100"`
}

type moviesListResponse struct {
	Movies   []movieObject  `json:"movies"`
	Metadata metadataObject `json:"metadata"`
}

type userObject struct {
	ID        int64  `json:"id" example:"1"`
	Name      string `json:"name" example:"Alice Smith"`
	Email     string `json:"email" example:"alice@example.com"`
	Activated bool   `json:"activated" example:"false"`
	CreatedAt string `json:"created_at" example:"2024-01-01T00:00:00Z"`
}

type userResponse struct {
	User userObject `json:"user"`
}

type authTokenObject struct {
	Token  string `json:"token" example:"ABCDEFGHIJKLMNOPQRSTUVWXYZ"`
	Expiry string `json:"expiry" example:"2024-01-02T00:00:00Z"`
}

type authTokenResponse struct {
	AuthenticationToken authTokenObject `json:"authentication_token"`
}

type messageResponse struct {
	Message string `json:"message" example:"operation completed successfully"`
}

type errorResponse struct {
	Error string `json:"error" example:"the requested resource could not be found"`
}

type validationErrorResponse struct {
	Error map[string]string `json:"error"`
}

type healthcheckResponse struct {
	Status     string            `json:"status" example:"available"`
	SystemInfo map[string]string `json:"system_info"`
}
