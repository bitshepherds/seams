const { Kafka } = require('kafkajs');

const producer = new Kafka({ brokers: ['kafka:9092'] }).producer();

async function accepted(order) {
  await producer.send({ topic: 'orders', messages: [{ value: JSON.stringify(order) }] });
}

module.exports = { accepted };
