# Use an official Python runtime as a parent image
FROM python:3.11-slim as base

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

# Set work directory
WORKDIR /app

# Install system dependencies (if any - add as needed)
# RUN apt-get update && apt-get install -y --no-install-recommends some-package && rm -rf /var/lib/apt/lists/*

# --- Build Stage ---
FROM base as builder

# Install build dependencies (if any - e.g., for compiling C extensions)
# RUN pip install --upgrade pip setuptools wheel

# Install Python dependencies
COPY requirements.txt .
RUN pip wheel --no-cache-dir --wheel-dir /app/wheels -r requirements.txt

# --- Final Stage ---
FROM base

# Create a non-root user
RUN adduser --system --group appuser

# Copy Python dependencies from builder stage
COPY --from=builder /app/wheels /wheels
COPY --from=builder /app/requirements.txt .
RUN pip install --no-cache /wheels/*

USER appuser

# Copy application code
COPY ./app ./app
COPY ./configs ./configs

# Expose port if needed (FastAPI default is 8000 with uvicorn)
# The README runs with uvicorn, which defaults to port 8000.
# Uncomment if the application directly serves HTTP requests that need exposure.
EXPOSE 8000

# Command to run the application
# We use the command from README but remove --reload for production
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
