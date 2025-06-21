import requests

token = "y0_asd"

headers = {
    "Authorization": f"OAuth {token}",
    "ya-token": token,
}

response = requests.get(
    "http://localhost:8000/get_current_track_beta", headers=headers
)

print(response.status_code)
print(response.text)
