include .env

#####################################
##  Helpers							#
#####################################
## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'


.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]


###########################################
## Development
###########################################
## run/api: runs the api
.PHONY: run/api
run/api:
	go run ./cmd/api

## db/migration/new: creates a new migration, the 'name' argument is used in the migration file created.
.PHONY: db/migrations/new
db/migration/new:
	goose -s create ${name} sql

## db/migration/up: run migrations
.PHONY: db/migrations/up
db/migration/up: confirm
	goose -dir ${GOOSE_MIGRATION_DIR} ${GOOSE_DRIVER}  ${GOOSE_DBSTRING} 


####################################
## QUALITY CONTROL
###################################
.PHONY: tidy
tidy:
	@echo 'Tidying module dependencies...'
	go mod tidy
	@echo 'Formatting .go files'
	go fmt ./...

.PHONY: audit
audit:
	@echo 'Checking Module dependencies...'
	go mod tidy -diff
	go mod verify
	@echo 'Vetting code..'
	go vet ./...
	go tool staticcheck ./...
	@echo 'Running test...'
	go test -race -vet=off ./...


################################################
## BUILD
################################################
#build/api: build the cmd/api application
.PHONY: build/api
build/api:
	go build -o=./bin/api ./cmd/api