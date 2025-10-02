from random import randrange
#import cv2  # Not used in this script, can be removed unless you plan to add image processing
import base64
import time
import requests
import json
from os import listdir
from os.path import isfile, join

accounts = []
rec_accounts = []

# Step 1: Create initial accounts
print("Account load...")
print("Enter amount of sender accounts:")
text = input(">>>  ")
for i in range(int(text)):
    data_req = {
        "method": "create_account",
        "jsonrpc": "2.0",
        "id": i + 1000,
        "params": [
            f"{i}",  # Account identifier or name
            f"{i}",  # Password or another parameter (adjust based on API requirements)
        ]
    }
    r = requests.post("http://localhost:1337/app", json=data_req)
    if r.status_code == 200:
        acc = json.loads(r.text)
        accounts.append(acc['result'])
        print(f"Created account {i}: {acc['result']['address']}")
    else:
        print(f"Failed to create account {i}: {r.text}")
    time.sleep(0.01)  # Small delay to avoid overwhelming the server

# Step 3: Create receiver accounts
print("Account receivers...")
print("Enter amount of receiver accounts:")
text = input(">>>  ")
for i in range(int(text)):
    data_req = {
        "method": "create_account",
        "jsonrpc": "2.0",
        "id": i + 2000,  # Different ID range to avoid conflicts
        "params": [
            f"rec_{i}",  # Differentiate receiver accounts
            f"rec_{i}",
        ]
    }
    r = requests.post("http://localhost:1337/app", json=data_req)
    if r.status_code == 200:
        acc = json.loads(r.text)
        rec_accounts.append(acc['result'])
        print(f"Created receiver account {i}: {acc['result']['address']}")
    else:
        print(f"Failed to create receiver account {i}: {r.text}")
    time.sleep(0.01)

# Step 4: Send funds from sender accounts to receiver accounts
print("Sending funds from senders to receivers...")
input("Press Enter to continue...")
if len(rec_accounts) == 0:
    print("No receiver accounts available to send to!")
else:
    for i, sender in enumerate(accounts):
        # Choose a receiver (cycle through rec_accounts if fewer than senders)
        receiver = rec_accounts[i % len(rec_accounts)]
        send_req = {
            "method": "send_tx",  # Adjust method name based on your API
            "jsonrpc": "2.0",
            "id": i + 3000,
            "params": [
                sender['pub'],     # From address
                receiver['address'],   # To address
                1.5,                   # Amount to send (adjust as needed)
                50000,
                f"Hi, i am a {sender['address']}"
            ]
        }
        r = requests.post("http://localhost:1337/app", json=send_req)
        if r.status_code == 200:
            print(f"Sent 1.5 from {sender['address']} to {receiver['address']}: {r.text}")
        else:
            print(f"Failed to send from {sender['address']} to {receiver['address']}: {r.text}")
        time.sleep(0.02)  # Slightly longer delay for transactions

print("Process completed!")