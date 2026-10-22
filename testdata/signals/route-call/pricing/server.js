const app = require('express')();

app.get('/quotes/:sku', (req, res) => res.json({ sku: req.params.sku }));
