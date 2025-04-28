# Notification Service

## Overview

This project implements a notification service composed of:

*   **Producer APIs** (implemented in Go and Python) that receive notification requests via HTTP and publish them to Kafka.
*   A **Consumer Service** (implemented in Go) that listens to Kafka, processes notifications, and dispatches them through configured channels.

The system is designed with observability (tracing, metrics, logging), configuration management, and graceful shutdown capabilities.

## Components

1.  **`go-producer` (Go Producer API)**
    *   Exposes an HTTP POST endpoint (`/send-notification`) using the Gin framework.
    *   Accepts notification requests containing content, target channels, destination, and user profile information.
    *   Marshals the notification data into JSON.
    *   Publishes the notification message to a configured Kafka topic.
    *   Integrates OpenTelemetry for tracing requests.
    *   Exposes Prometheus metrics on `/metrics`.
    *   Uses Zap for structured logging.

2.  **`py-producer` (Python Producer API)**
    *   Provides similar functionality to the Go producer.
    *   Exposes an HTTP POST endpoint (`/send-notification`) using the FastAPI framework.
    *   Accepts notification requests (schema defined using Pydantic).
    *   Publishes messages to a configured Kafka topic using `confluent-kafka`.
    *   Integrates OpenTelemetry for tracing requests (using `opentelemetry-instrumentation-fastapi`).
    *   Exposes Prometheus metrics on `/metrics` (using `prometheus-client`).
    *   Uses Structlog for structured logging.

3.  **`consumers` (Go Consumer Service)**
    *   Listens to a configured Kafka topic and group ID.
    *   Uses a worker pool (controlled by a semaphore) to process messages concurrently.
    *   Unmarshals notification messages from Kafka.
    *   Extracts OpenTelemetry trace context from message headers.
    *   Implements a channel registry pattern to dynamically load and use configured notification channels (e.g., email).
    *   Dispatches notifications via the appropriate channel implementation (currently includes an SMTP email channel).
    *   Handles message processing lifecycle: Ack, Retry (with exponential backoff + jitter), and Move to DLQ (Dead Letter Queue).
    *   Implements graceful shutdown handling OS signals (SIGINT, SIGTERM).
    *   Exposes Prometheus metrics on `/metrics` (runs on a separate port).
    *   Uses Zap for structured logging.

## Features

*   **Decoupled Architecture:** Producers and Consumer are separate services communicating via Kafka.
*   **Multiple Producer Implementations:** Demonstrates producer logic in both Go and Python.
*   **Multiple Channel Support:** Designed to support different notification channels (Email implemented).
*   **Configuration Driven:** Uses `.env` files for configuration (Viper in Go, Pydantic Settings/python-dotenv in Python).
*   **Observability:**
    *   **Structured Logging:** Using Zap (Go) and Structlog (Python).
    *   **Distributed Tracing:** Using OpenTelemetry (OTLP GRPC exporter).
    *   **Metrics:** Using Prometheus (exposed via HTTP endpoint).
*   **Resiliency (Consumer):** Includes retry logic with exponential backoff and jitter, and a Dead Letter Queue mechanism for failed messages.
*   **Concurrency (Consumer):** Consumer uses a worker pool to process messages concurrently.
*   **Graceful Shutdown (Consumer):** Handles OS signals for clean shutdown.

## Technology Stack

*   **Languages:** Go, Python
*   **Messaging:** Kafka (`segmentio/kafka-go` for Go, `confluent-kafka` for Python)
*   **API Frameworks:** Gin (Go), FastAPI (Python)
*   **Configuration:** Viper (Go), Pydantic Settings & python-dotenv (Python)
*   **Logging:** Zap (Go), Structlog (Python)
*   **Tracing:** OpenTelemetry (OTLP)
*   **Metrics:** Prometheus Client (`prometheus/client_golang`, `prometheus-client`)
*   **Email (Consumer):** Standard Go `net/smtp` package

## Configuration

Configuration for all services is managed primarily via environment variables set within the `docker-compose.yml` file. For sensitive information like email credentials, Docker secrets are used.

Key environment variables configured in `docker-compose.yml` include:

*   `KAFKA_BROKERS`: List of Kafka broker addresses (e.g., `kafka:9092`).
*   `KAFKA_TOPIC`: The main Kafka topic for notifications.
*   `KAFKA_GROUP_ID` (Consumer): The Kafka consumer group ID.
*   `KAFKA_DLQ_TOPIC` (Consumer): The topic for dead-lettered messages.
*   `ENABLED_CHANNELS` (All Services): Comma-separated list of channels to activate (e.g., `email,sms`). Format might differ slightly per service (e.g., JSON array for Python Producer `["sms", "email"]`).
*   `WORKER_POOL_SIZE` (Consumer): Number of concurrent message processors.
*   `MAX_RETRIES` (Consumer): Maximum number of times to retry a failed message.
*   `BACKOFF_BASE_DELAY_MS` (Consumer): Base delay for exponential backoff in milliseconds.
*   `OTEL_SERVICE_NAME`: Service name for OpenTelemetry.
*   `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP exporter endpoint (e.g., `jaeger:4317` or `http://jaeger:4317`).
*   `OTEL_EXPORTER_OTLP_INSECURE`: Set to `true` to disable TLS for OTLP export.
*   `METRICS_SERVER_ADDRESS` (Consumer): Address for the Prometheus metrics HTTP server (e.g., `:8081`).

**Email Secrets (Consumer):** Email credentials for the `notification-consumer` service are managed via a Docker secret.

*   The file `email_secrets.env` (located in the project root) contains the following key-value pairs:
    *   `EMAIL_DRIVER`, `EMAIL_MAILER`, `EMAIL_HOST`, `EMAIL_PORT`, `EMAIL_USERNAME`, `EMAIL_PASSWORD`, `EMAIL_ENCRYPTION`, `EMAIL_FROM_ADDRESS`, `EMAIL_FROM_NAME`.
*   This file is mounted as the `email_secrets` Docker secret.
*   The `notification-consumer` service uses the `env_file` directive in `docker-compose.yml` to load these values from the mounted secret (`/run/secrets/email_secrets`) into its environment.
*   **Modify the `email_secrets.env` file directly** if you need to change email settings.

## Running the Services

This project is designed to be run using Docker Compose, which orchestrates the producers, consumer, Kafka, Zookeeper, Prometheus, Grafana, and Jaeger containers.

**Prerequisites:**

*   Docker and Docker Compose installed.

**Steps:**

1.  **Configuration:**
    *   Review and adjust non-sensitive environment variables directly within the `docker-compose.yml` file if needed (e.g., Kafka endpoints).
    *   **Important:** Configure email credentials by editing the `email_secrets.env` file in the project root directory. **Do not commit sensitive credentials** if sharing the repository.

2.  **Build and Run:** Navigate to the root directory of the project and run:

    ```bash
    docker-compose up --build -d
    ```

3.  **Accessing Services:** (Assuming default ports and docker-compose integration)
    *   **Go Producer API:** `http://localhost:8080`
        *   **API Docs (Swagger UI):** `http://localhost:8080/docs/index.html`
    *   **Python Producer API:** `http://localhost:8000`
        *   **API Docs (Swagger UI):** `http://localhost:8000/docs`
    *   **Prometheus:** `http://localhost:9090`
    *   **Grafana:** `http://localhost:3000` (Login: admin/admin)
    *   **Jaeger UI:** `http://localhost:16686`
    *   **Kafka:** Accessible internally at `kafka:9092` and externally at `localhost:29092`.

4.  **Viewing Logs:** To view the logs for a specific service:

    ```bash
    docker-compose logs -f <service_name>
    # Example: docker-compose logs -f notification-producer-go
    # Example: docker-compose logs -f notification-producer-py
    # Example: docker-compose logs -f notification-consumer
    ```

5.  **Stopping Services:** To stop and remove the containers:

    ```bash
    docker-compose down
    ```

## Observability

*   **Logs:** Services output structured logs. Check container logs via `docker-compose logs`.
*   **Metrics:** Prometheus metrics are available at:
    *   Go Producer: `http://<go_producer_host>:8080/metrics`
    *   Python Producer: `http://<py_producer_host>:<PY_PRODUCER_PORT>/metrics`
    *   Consumer: `http://<consumer_host>:<METRICS_SERVER_ADDRESS>/metrics` (e.g., `http://localhost:8081/metrics`)
*   **Tracing:** Services export traces via OTLP GRPC to the configured `OTEL_EXPORTER_OTLP_ENDPOINT`. View traces in Jaeger (`http://localhost:16686`).

## Testing

Run tests using standard tooling within each service directory:

```bash
# Go services
cd go-producer && go test ./...
cd ../consumers && go test ./...

# Python service (Example using pytest, adjust if needed)
cd ../py-producer
pip install pytest
pytest
``` 
