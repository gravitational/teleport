FROM python:3.11-slim
RUN pip install --no-cache-dir cryptography pyjwt
COPY server.py /app/server.py
EXPOSE 8080
CMD ["python3", "-u", "/app/server.py"]
