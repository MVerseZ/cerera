from random import randrange
import cv2
import base64
import time
import requests
import json
from os import listdir
from os.path import isfile, join

accounts = []

print("Account load...")
print("Enter amount:")
text = input(">>>  ")
for i in range(int(text)):
    data_req = {
        "method": "create_account",
        "jsonrpc": "2.0",
        "id": i+1000,
        "params":[
            f"{i}",
            f"{i}",
        ]
    }
    r = requests.post("http://localhost:1337/app", json=data_req)
    # print(r.text)
    acc = json.loads(r.text)
    accounts.append(acc['result'])
 
# print("Faucet testing...")
# input("Press Enter to continue...")
       
# for acc in accounts:
#     faucet_req = {
#         "method": "faucet",
#         "jsonrpc": "2.0",
#         "id": i+1000,
#         "params":[
#             acc['address'],
#             3.75,
#         ]
#     }
#     r = requests.post("http://localhost:1337/app", json=faucet_req)
#     print(r.text)

# data_req = {
#     "method": "accounts",
#     "jsonrpc": "2.0",
#     "id": 1000
# }

# print("Sync testing...")
# input("Press Enter to continue...")

# r = requests.post("http://localhost:1337/app", json=data_req)
# #print(r.text)
# data = json.loads(r.text)
# print(data['result'])

# r2 = requests.post("http://localhost:1339/app", json=data_req)
# #print(r.text)
# data2 = json.loads(r2.text)
# print(data2['result'])

# print(data==data2)


print("Pool testing...")
input("Press Enter to continue...")

pool_req = {
    "method": "getmempoolinfo",
    "jsonrpc": "2.0",
    "id": 1000
}

r = requests.post("http://localhost:1337/app", json=pool_req)
#print(r.text)
data = json.loads(r.text)
# print(data['result'])

r2 = requests.post("http://localhost:1339/app", json=pool_req)
#print(r.text)
data2 = json.loads(r2.text)
# print(data2['result'])

print(data==data2)



