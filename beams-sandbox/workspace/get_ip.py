#!/usr/bin/env python3
import urllib.request
import json

def get_public_ip():
    """Get the public IP address using ipify API"""
    try:
        with urllib.request.urlopen('https://api.ipify.org?format=json', timeout=5) as response:
            data = json.loads(response.read().decode())
            return data['ip']
    except Exception as e:
        return f"Error fetching IP: {e}"

if __name__ == "__main__":
    ip_address = get_public_ip()
    print(f"Your public IP address is: {ip_address}")
