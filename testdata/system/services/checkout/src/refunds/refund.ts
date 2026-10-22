import Stripe from 'stripe';

export const refund = (key: string, id: string) => new Stripe(key).refunds.create({ payment_intent: id });
