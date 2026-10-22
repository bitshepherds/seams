import express from 'express';

import { place } from './orders';

const app = express();
app.post('/orders', place);
app.listen(3000);
