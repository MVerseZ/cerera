from random import randrange
import cv2
import base64
import time
import requests
from os import listdir
from os.path import isfile, join
path = "D:/Pictures/tmp_vid/w"
onlyfiles = [f for f in listdir(path) if isfile(join(path, f))]

enc_f = []
for f in onlyfiles:
    # with open(f"{path}/{f}", "rb") as image_file:
    img = cv2.imread(f"{path}/{f}")        
    jpg_img = cv2.imencode('.jpg', img)
    b64_string = base64.b64encode(jpg_img[1]).decode('utf-8')
    enc_f.append(b64_string)       

pref = "data:image/jpeg;base64,"

# {
#     "jsonrpc": "2.0",
#     "result": {
#         "address": "0xa3d3d7ca0234bb20fa7530a183db2d1ca953d56e58314ff17f1cb8e7b4e6522df970e2a9932c6e08b0dd3517d0be9197",
#         "pub": "xpub661MyMwAqRbcGiPeHEGL2LAKLfxrKQYq6cpadnK3aBumzjxhgYXndado2XPvZ7FaYk5xGcoyyLF97FJSV1Xj6dN8MStuPbWG4ikJwVSf64V",
#         "mnemonic": "tragic slender goddess sound muffin patrol cool coil garment swift unique emerge paddle scare forum myth tonight milk mystery orchard rookie tent remain ski"
#     },
#     "id": 11231
# }

# Create sender account
sender_req = {
    "method": "cerera.account.create",
    "jsonrpc": "2.0",
    "id": 1001,
    "params": ["sender", "password123"]
}
sender_resp = requests.post("http://localhost:1337/app", json=sender_req)
sender_data = sender_resp.json()["result"]

# Create receiver account
receiver_req = {
    "method": "cerera.account.create",
    "jsonrpc": "2.0",
    "id": 1002,
    "params": ["receiver", "password456"]
}
receiver_resp = requests.post("http://localhost:1337/app", json=receiver_req)
receiver_data = receiver_resp.json()["result"]

print(f"Sender address: {sender_data['address']}")
print(f"Receiver address: {receiver_data['address']}")

for i in range(100):
    ii = randrange(len(enc_f))
    data = pref+enc_f[ii]
    data_req = {
        "method": "cerera.transaction.send",
        "jsonrpc": "2.0",
        "id": i+1000,
        "params":[
            sender_data['pub'],  # Use sender's private key
            receiver_data['address'],  # Use receiver's address
            10,
            1,
            data,
        ]
    }
    r = requests.post("http://localhost:1337/app", json=data_req)
    print(f"Tx {i}: {r.text}")
    if i % 1000 == 0:
        time.sleep(13)
    # time.sleep(0.001)

# import base64

# with open("yourfile.ext", "rb") as image_file:
#     encoded_string = base64.b64encode(image_file.read()).


# for i in range(11):
#     data_req = {
#         "method": "create_account",
#         "jsonrpc": "2.0",
#         "id": i+1000,
#         "params":[
#             f"{i}",
#             f"{i}",
#         ]
#     }
#     r = requests.post("http://localhost:1337/app", json=data_req)
#     print(r.text)