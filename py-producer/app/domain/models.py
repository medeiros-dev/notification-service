from pydantic import BaseModel, Field, HttpUrl
from typing import List, Optional, Dict, Any
import datetime
import uuid

class Channel(BaseModel):
    type: str = Field(..., description="Type of the channel (e.g., email, sms, push)")
    destination: str = Field(..., description="Destination address/identifier for the channel")
    enabled: bool = Field(True, description="Whether this channel is enabled for the notification")

class UserProfile(BaseModel):
    user_id: str = Field(..., description="Unique identifier for the user")
    name: Optional[str] = Field(None, description="User's full name")
    preferred_language: Optional[str] = Field(None, description="User's preferred language (e.g., pt-BR, en-US)")
    timezone: Optional[str] = Field(None, description="User's timezone (e.g., America/Sao_Paulo)")

class Metadata(BaseModel):
    priority: Optional[str] = Field(None, description="Priority of the notification (e.g., high, medium, low)")
    category: Optional[str] = Field(None, description="Category of the notification (e.g., system_alert, marketing)")
    timestamp: Optional[datetime.datetime] = Field(None, description="Timestamp for the metadata")

class SendNotificationRequest(BaseModel):
    id: uuid.UUID = Field(default_factory=uuid.uuid4, description="Unique identifier for the notification")
    content: str = Field(..., description="The main content of the notification")
    channels: List[Channel] = Field(..., description="List of channels to send the notification through")
    user_profile: Optional[UserProfile] = Field(None, description="User profile information relevant to the notification")
    metadata: Optional[Metadata] = Field(None, description="Additional metadata for the notification")

    model_config = {
        "json_schema_extra": {
            "examples": [
                {
                  "id": "01966308-7c93-7116-b009-2dc3f5f782aa",
                  "content": "Example notification content.",
                  "channels": [
                    {
                      "type": "email",
                      "destination": "example@example.com",
                      "enabled": True
                    },
                    {
                      "type": "sms",
                      "destination": "+15551234567",
                      "enabled": False
                    }
                  ],
                  "user_profile": {
                    "user_id": "user_123",
                    "name": "John Doe",
                    "preferred_language": "en-US",
                    "timezone": "America/New_York"
                  },
                  "metadata": {
                    "priority": "medium",
                    "category": "account_update",
                    "timestamp": "2024-05-20T10:00:00Z"
                  }
                }
            ]
        }
    }

class Notification(BaseModel):
    id: str
    content: str
    channel_type: str
    destination: str
    user_profile: UserProfile
    created_at: datetime.datetime | None = None 