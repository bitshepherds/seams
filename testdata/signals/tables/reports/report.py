import psycopg


def shipped(conn: psycopg.Connection):
    return conn.execute("SELECT order_id FROM shipments").fetchall()
