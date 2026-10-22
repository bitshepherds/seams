import csv

import grpc
import psycopg

import pricing_pb2_grpc


def run(conn: psycopg.Connection, target: str, out: str):
    rows = conn.execute("SELECT order_id, shipped_at FROM shipments").fetchall()
    pricing = pricing_pb2_grpc.PricingStub(grpc.insecure_channel(target))
    with open(out, "w", newline="") as f:
        csv.writer(f).writerows(rows)
    return pricing
