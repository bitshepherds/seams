import requests


def quote(sku):
    return requests.get("http://pricing:8080/quotes/" + sku).json()
