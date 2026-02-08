FROM python:3.12-slim

# Set working directory
WORKDIR /bot

# Copy bot files
COPY . /bot

# Install dependencies if requirements.txt exists
RUN pip install --no-cache-dir -r requirements.txt || true

# Run the bot (unbuffered output)
ENTRYPOINT ["python", "-u", "bot.py"]
