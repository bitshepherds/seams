resource "aws_sns_topic" "orders" {
  name = "orders"
}

resource "aws_lambda_function" "fulfilment" {
  function_name = "fulfilment"
  filename      = "../fulfilment/dist.zip"
  handler       = "app.handler"
  runtime       = "python3.12"
  role          = aws_iam_role.lambda.arn
}

resource "aws_sns_topic_subscription" "fulfilment" {
  topic_arn = aws_sns_topic.orders.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.fulfilment.arn
}
