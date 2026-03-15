# =============================================
# LAMBDA FUNCTION — Using LabRole (no IAM role creation)
# =============================================

resource "aws_lambda_function" "order_processor" {
  function_name = "order-processor-lambda"
  role          = var.lab_role_arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  memory_size   = 512
  timeout       = 30

  filename         = "${path.module}/../lambda/function.zip"
  source_code_hash = filebase64sha256("${path.module}/../lambda/function.zip")
}

# Subscribe Lambda to SNS topic
resource "aws_sns_topic_subscription" "lambda_sub" {
  topic_arn = aws_sns_topic.order_events.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor.arn
}

# Allow SNS to invoke Lambda
resource "aws_lambda_permission" "allow_sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.order_events.arn
}

# CloudWatch log group for Lambda
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${aws_lambda_function.order_processor.function_name}"
  retention_in_days = 7
}