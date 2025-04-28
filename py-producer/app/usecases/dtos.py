from pydantic import BaseModel
from datetime import datetime
from typing import List, Optional
from app.domain.models import UserProfile

class Channel(BaseModel):
    type: str
    destination: str
    enabled: bool = True

class SendNotificationInputDTO(BaseModel):
    id: str
    content: str
    channels: List[Channel]
    user_profile: UserProfile
    created_at: Optional[datetime] = None

class MessageResponse(BaseModel):
    id: str
    channel_type: str
    status: str
    error: Optional[str] = None

class SendNotificationOutputDTO(BaseModel):
    messages: List[MessageResponse] 