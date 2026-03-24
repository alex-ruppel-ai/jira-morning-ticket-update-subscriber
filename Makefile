.PHONY: install deps backend frontend run build deploy clean tidy test test-unit test-integration test-e2e test-all

install:
	cd frontend && npm install

deps:
	go mod tidy
	go mod download
	cd frontend && npm install

tidy:
	go mod tidy

backend:
	DEV=true go run .

frontend:
	cd frontend && npm run dev

build-frontend:
	cd frontend && npm run build

run: deps
	@echo "Starting Go backend and React frontend in dev mode..."
	@echo "Backend will run on http://localhost:8081"
	@echo "Frontend will run on http://localhost:3000"
	@make -j2 backend frontend

build: deps
	cd frontend && npm run build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

deploy:
	apps-platform app deploy

clean:
	cd frontend && rm -rf node_modules dist
	rm -f app
	go clean

-include .env
export

test-unit:
	go test ./... -v

test-e2e:
	go test -tags=e2e -v

test-all: test-unit test-e2e

test: test-unit
