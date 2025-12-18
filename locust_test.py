from locust import HttpUser, task, constant
import json
import random
from datetime import datetime

class StreamUser(HttpUser):
    wait_time = constant(0)

    @task
    def send_stream_data(self):
        payload = {
            "device_id": f"device_{random.randint(1, 20)}",
            "timestamp": int(datetime.now().timestamp()),
            "cpu": round(random.uniform(0.0, 100.0), 2),
            "rps": random.randint(1, 1000)
        }
        self.client.post("/stream", json=payload)