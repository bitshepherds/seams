resource "aws_lambda_function" "checkout" {
  function_name = "checkout"
  filename      = "../checkout/dist.zip"
  handler       = "index.handler"
  runtime       = "nodejs20.x"
  role          = aws_iam_role.lambda.arn
}

resource "aws_apigatewayv2_api" "public" {
  name          = "public"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_integration" "checkout" {
  api_id           = aws_apigatewayv2_api.public.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.checkout.invoke_arn
}
