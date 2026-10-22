const Stripe = require('stripe');

module.exports = (key) => new Stripe(key);
