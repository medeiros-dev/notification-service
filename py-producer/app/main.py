import logging
import structlog
from fastapi import FastAPI, Request, status
from fastapi.responses import JSONResponse
from configs.config import settings
from app.domain.models import Notification, SendNotificationRequest
from app.usecases.dtos import SendNotificationInputDTO, SendNotificationOutputDTO
from app.usecases.send_notification import SendNotificationUseCase
from app.infrastructure.kafka_broker import KafkaBroker
from app.observability.metrics import MetricsMiddleware, metrics_endpoint, messages_received_total, messages_sent_success_total, messages_sent_failed_total, error_total
from app.observability.tracing import init_tracing

# Logging setup
logging.basicConfig(level=logging.INFO, format="%(message)s")
structlog.configure(
    wrapper_class=structlog.make_filtering_bound_logger(logging.INFO),
    processors=[
        structlog.processors.JSONRenderer()
    ]
)
logger = structlog.get_logger()

app = FastAPI(title="Python Notification Producer")

# Observability
app.add_middleware(MetricsMiddleware)
init_tracing(app)

# Kafka broker and use case
broker = KafkaBroker(settings.kafka_brokers, settings.kafka_topic)
use_case = SendNotificationUseCase(broker, settings.enabled_channels)

@app.post("/send-notification", 
          response_model=SendNotificationOutputDTO, 
          summary="Send a notification", 
          description="Accepts notification details and publishes them to the appropriate Kafka topic.")
async def send_notification(payload: SendNotificationRequest, request: Request):
    input_dto = SendNotificationInputDTO(
        id=str(payload.id),
        content=payload.content,
        channels=[c.model_dump() for c in payload.channels], # Convert Channel objects
        user_profile=payload.user_profile.model_dump() if payload.user_profile else None, # Convert UserProfile
        metadata=payload.metadata.model_dump() if payload.metadata else None # Convert Metadata
    )

    # Metrics: count received messages per channel
    for channel in input_dto.channels:
        messages_received_total.labels(channel.type).inc() 
    try:
        output = use_case.execute(input_dto, context=request)
        # Metrics: count success/error per channel
        for msg in output.messages:
            if msg.status == "success":
                messages_sent_success_total.labels(msg.channel_type).inc()
            else:
                messages_sent_failed_total.labels(msg.channel_type).inc()
        return output
    except Exception as e:
        logger.error("Failed to process notification", error=str(e))
        error_total.labels("send_notification").inc()
        return JSONResponse(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, content={"error": "Failed to process notification"})

@app.get("/metrics", include_in_schema=False)
def metrics():
    return metrics_endpoint() 