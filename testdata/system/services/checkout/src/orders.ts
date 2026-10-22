import { Kafka } from 'kafkajs';
import { add } from '@acme/money';

const producer = new Kafka({ brokers: ['kafka:9092'] }).producer();

export async function place(req, res) {
  const total = add(req.body.items);
  await producer.send({ topic: 'orders', messages: [{ value: JSON.stringify({ total }) }] });
  res.status(201).end();
}
