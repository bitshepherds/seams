export async function quote(sku: string): Promise<number> {
  const res = await fetch(`${process.env.PRICING_URL}/quotes/${sku}`);
  return (await res.json()).price;
}
