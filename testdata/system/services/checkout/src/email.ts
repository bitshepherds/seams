import { SQSClient, SendMessageCommand } from '@aws-sdk/client-sqs';

const sqs = new SQSClient({});

export const receipt = (to: string) =>
  sqs.send(new SendMessageCommand({ QueueUrl: process.env.EMAILS_QUEUE_URL, MessageBody: to }));
