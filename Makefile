.PHONY: build dev clean db-migrate docker-up docker-down

# Build everything
build:
	@echo "Building backend..."
	cd backend && go build -o bin/api ./cmd/api
	@echo "Backend built successfully"
	@echo "Frontend uses Expo - no build needed for development"

# Development mode - start all services
dev:
	@echo "Starting development environment..."
	docker-compose up -d postgres ejabberd
	@echo "PostgreSQL and ejabberd started"
	@echo "Starting backend..."
	cd backend && ./bin/api &
	@echo "Backend started on :8080"
	@echo "Starting frontend..."
	cd client && npm start

# Apply database migrations
db-migrate:
	@echo "Applying database migrations..."
	cd backend && psql postgresql://postgres:postgres@localhost:5432/corp_messenger -f migrations/007_notification_settings.sql
	cd backend && psql postgresql://postgres:postgres@localhost:5432/corp_messenger -f migrations/022_protocol_preference.sql
	@echo "Migrations applied"

# Start Docker services only
docker-up:
	docker-compose up -d postgres ejabberd

# Stop Docker services
docker-down:
	docker-compose down

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd backend && rm -f bin/api
	cd client && rm -rf node_modules
	@echo "Cleaned"

# Install frontend dependencies
install-deps:
	cd client && npm install
