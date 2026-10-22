async function quote(sku) {
  const res = await fetch(`${process.env.PRICING_BASE}/quotes/${sku}`);
  return res.json();
}

module.exports = { quote };
