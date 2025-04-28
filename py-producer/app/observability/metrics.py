from prometheus_client import Counter, Histogram, generate_latest
from fastapi import Request, Response
from starlette.middleware.base import BaseHTTPMiddleware
import time

http_requests_total = Counter(
    "py_producer_http_requests_total",
    "Total number of HTTP requests processed, labeled by endpoint and status code.",
    ["endpoint", "status"]
)

http_request_duration = Histogram(
    "py_producer_http_request_duration_seconds",
    "Histogram of latencies for HTTP requests.",
    ["endpoint"]
)

kafka_publish_total = Counter(
    "py_producer_kafka_publish_total",
    "Total number of Kafka publish attempts, labeled by result.",
    ["result"]
)

kafka_publish_duration = Histogram(
    "py_producer_kafka_publish_duration_seconds",
    "Histogram of Kafka publish durations."
)

error_total = Counter(
    "py_producer_error_total",
    "Total number of errors, labeled by type.",
    ["type"]
)

messages_received_total = Counter(
    "py_producer_messages_received_total",
    "Total number of messages received for processing, labeled by channel_type.",
    ["channel_type"]
)

messages_sent_success_total = Counter(
    "py_producer_messages_sent_success_total",
    "Total number of messages sent successfully to Kafka, labeled by channel_type.",
    ["channel_type"]
)

messages_sent_failed_total = Counter(
    "py_producer_messages_sent_failed_total",
    "Total number of messages failed to send to Kafka, labeled by channel_type.",
    ["channel_type"]
)

class MetricsMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next):
        endpoint = request.url.path
        if endpoint == "/metrics":
            return await call_next(request)
        start = time.time()
        response: Response = await call_next(request)
        status = str(response.status_code)
        http_requests_total.labels(endpoint, status).inc()
        http_request_duration.labels(endpoint).observe(time.time() - start)
        return response

def metrics_endpoint():
    return Response(generate_latest(), media_type="text/plain") 