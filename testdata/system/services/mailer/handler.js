module.exports.send = async (event) => event.Records.map((r) => r.body);
