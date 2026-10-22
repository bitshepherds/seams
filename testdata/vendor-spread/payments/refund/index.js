import Stripe from 'stripe';

export default (key) => new Stripe(key).refunds;
