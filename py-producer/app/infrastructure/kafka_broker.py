import json
from confluent_kafka import Producer
from opentelemetry import trace, propagate
from opentelemetry.trace import get_current_span
from app.domain.models import Notification
from configs.config import settings
import logging

logger = logging.getLogger("py-producer.kafka")

class KafkaBroker:
    def __init__(self, brokers: list[str], topic: str):
        self.producer = Producer({"bootstrap.servers": ",".join(brokers)})
        self.topic = topic

    def send_with_context(self, notification: Notification, context=None):
        # Prepare message
        value = notification.model_dump_json().encode()
        key = notification.id.encode()
        headers = []
        # Inject OTEL trace context
        carrier = {}
        propagate.inject(carrier)
        for k, v in carrier.items():
            headers.append((k, v.encode() if isinstance(v, str) else v))
        try:
            self.producer.produce(
                topic=self.topic,
                key=key,
                value=value,
                headers=headers,
            )
            self.producer.flush()
            logger.info(f"Notification sent: {notification.id}")
            return True, None
        except Exception as e:
            logger.error(f"Failed to send notification {notification.id}: {e}")
            return False, str(e) 