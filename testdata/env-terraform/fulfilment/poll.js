const { SQSClient, ReceiveMessageCommand } = require('@aws-sdk/client-sqs');

const client = new SQSClient({});

module.exports = () => client.send(new ReceiveMessageCommand({ QueueUrl: process.env.INBOX_URL }));
