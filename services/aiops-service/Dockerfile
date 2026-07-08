FROM python:3.12.13-slim-bookworm AS runtime

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY pyproject.toml ./
RUN pip install --no-cache-dir .

COPY app ./app

EXPOSE 8080
RUN groupadd --system --gid 65532 appuser \
    && useradd --system --uid 65532 --gid 65532 --home-dir /app appuser
USER 65532:65532
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080"]
