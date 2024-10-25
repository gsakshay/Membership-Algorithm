
DOCKER_IMAGE = prj3
DOCKER_COMPOSE_FILES = docker-compose-testcase-1.yml docker-compose-testcase-2.yml docker-compose-testcase-3.yml docker-compose-testcase-4.yml

.PHONY: all build run clean stop

# Default target
all: build run

# Build the Docker image
build:
	docker build -t $(DOCKER_IMAGE) .

# Run the project with a specific Docker Compose file
run:
	@if [ -z "$(TEST_CASE)" ]; then \
		echo "Please specify TEST_CASE=<number> (1-4)"; \
		exit 1; \
	fi; \
	if [ $(TEST_CASE) -eq 1 ]; then \
		docker compose -f docker-compose-testcase-1.yml up -d; \
	elif [ $(TEST_CASE) -eq 2 ]; then \
		docker compose -f docker-compose-testcase-2.yml up -d; \
	elif [ $(TEST_CASE) -eq 3 ]; then \
		docker compose -f docker-compose-testcase-3.yml up -d; \
	elif [ $(TEST_CASE) -eq 4 ]; then \
		docker compose -f docker-compose-testcase-4.yml up -d; \
	else \
		echo "Invalid TEST_CASE value. Please use 1, 2, 3, or 4."; \
		exit 1; \
	fi

# Stop the running Docker containers for a specific test case
stop:
	@if [ -z "$(TEST_CASE)" ]; then \
		echo "Please specify TEST_CASE=<number> (1-4)"; \
		exit 1; \
	fi; \
	if [ $(TEST_CASE) -eq 1 ]; then \
		docker compose -f docker-compose-testcase-1.yml stop; \
	elif [ $(TEST_CASE) -eq 2 ]; then \
		docker compose -f docker-compose-testcase-2.yml stop; \
	elif [ $(TEST_CASE) -eq 3 ]; then \
		docker compose -f docker-compose-testcase-3.yml stop; \
	elif [ $(TEST_CASE) -eq 4 ]; then \
		docker compose -f docker-compose-testcase-4.yml stop; \
	else \
		echo "Invalid TEST_CASE value. Please use 1, 2, 3, or 4."; \
		exit 1; \
	fi

# Clean up Docker containers, images, and volumes
clean:
	@for file in $(DOCKER_COMPOSE_FILES); do \
		docker compose -f $$file down --volumes; \
	done
	docker rmi $(DOCKER_IMAGE)

# Rebuild and run the project
rebuild: clean build run
