FROM python:3-slim
WORKDIR /app
COPY server.py .
ENV APP_HOST=0.0.0.0
EXPOSE 8000
CMD ["python", "server.py", "8000"]
