from app.usecases.dtos import SendNotificationInputDTO, SendNotificationOutputDTO, MessageResponse
from app.domain.models import Notification
from app.infrastructure.kafka_broker import KafkaBroker
from uuid import uuid4
from typing import Any

class SendNotificationUseCase:
    def __init__(self, broker: KafkaBroker, enabled_channels: list[str]):
        self.broker = broker
        self.enabled_channels = enabled_channels

    def execute(self, input_dto: SendNotificationInputDTO, context: Any = None) -> SendNotificationOutputDTO:
        notifications = []
        for channel in input_dto.channels:
            if channel.type not in self.enabled_channels or not channel.enabled:
                continue
            notification = Notification(
                id=str(uuid4()),
                content=input_dto.content,
                channel_type=channel.type,
                destination=channel.destination,
                user_profile=input_dto.user_profile,
            )
            success, error = self.broker.send_with_context(notification, context)
            if success:
                notifications.append(MessageResponse(
                    id=notification.id,
                    channel_type=notification.channel_type,
                    status="success"
                ))
            else:
                notifications.append(MessageResponse(
                    id=notification.id,
                    channel_type=notification.channel_type,
                    status="error",
                    error=error
                ))
        return SendNotificationOutputDTO(messages=notifications) 