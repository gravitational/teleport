FROM python
WORKDIR /app
COPY requirements.txt /app/
COPY teleport_usage.py /app/
RUN pip install --upgrade pip setuptools
RUN pip install -r requirements.txt
ENTRYPOINT ["python", "/app/teleport_usage.py"]
