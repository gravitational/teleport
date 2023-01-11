FROM python:3.11.1-alpine3.17
WORKDIR /app
RUN python -m pip install --upgrade pip setuptools
COPY requirements.txt /app/
RUN pip install -r requirements.txt
COPY teleport_usage.py /app/
ENTRYPOINT ["python", "/app/teleport_usage.py"]
