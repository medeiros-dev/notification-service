from pydantic_settings import BaseSettings
from pydantic import Field
from typing import List

class Settings(BaseSettings):
    kafka_brokers: List[str] = Field(..., env="KAFKA_BROKERS")
    kafka_topic: str = Field(..., env="KAFKA_TOPIC")
    enabled_channels: List[str] = Field(..., env="ENABLED_CHANNELS")
    otel_exporter_otlp_endpoint: str = Field(..., env="OTEL_EXPORTER_OTLP_ENDPOINT")
    otel_service_name: str = Field(..., env="OTEL_SERVICE_NAME")
    otel_exporter_otlp_insecure: bool = Field(False, env="OTEL_EXPORTER_OTLP_INSECURE")

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        case_sensitive = False

settings = Settings() 