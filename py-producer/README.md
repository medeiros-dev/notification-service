# Python Notification Producer (`py-producer`)

This service replicates the functionality of the Go-based `go-producer`, providing a REST API to send notifications via Kafka, with observability (Prometheus, OpenTelemetry) and structured logging.

## Features
- FastAPI HTTP API (`/send-notification`)
- Kafka producer with OTEL trace context propagation
- Prometheus metrics
- OpenTelemetry tracing
- Structured logging
- Configurable via `.env`

## Project Structure
```
py-producer/
  ├── app/
  │   ├── domain/
  │   ├── infrastructure/
  │   ├── usecases/
  │   ├── observability/
  │   └── main.py
  ├── configs/
  │   └── config.py
  ├── requirements.txt
  ├── .env
  └── README.md
```

## Running
1. Install dependencies: `pip install -r requirements.txt`
2. Set up `.env` (see example in Go version)
3. Run: `uvicorn app.main:app --reload`

## Testing
- Unit and integration tests will be in `tests/` (to be added).

## Notes
- Kafka, Prometheus, and OTEL endpoints must be available as per `.env`.
- This service is designed for extensibility and observability. 