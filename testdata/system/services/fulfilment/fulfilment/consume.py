from confluent_kafka import Consumer

consumer = Consumer({"bootstrap.servers": "kafka:9092", "group.id": "fulfilment"})
consumer.subscribe(["orders"])
