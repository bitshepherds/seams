const base = process.env.PRICING_URL;

module.exports = (sku) => fetch(base + sku);
