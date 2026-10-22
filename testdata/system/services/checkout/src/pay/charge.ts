import Stripe from 'stripe';

export const charge = (key: string, amount: number) => new Stripe(key).paymentIntents.create({ amount, currency: 'gbp' });
