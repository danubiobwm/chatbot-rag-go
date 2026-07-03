#!/bin/bash
# Roda automaticamente quando o container do LocalStack sobe
# (montado em /etc/localstack/init/ready.d).
set -e

awslocal s3 mb s3://chatbot-regulatorio-docs

awslocal sqs create-queue --queue-name extraction-queue-dlq
DLQ_ARN=$(awslocal sqs get-queue-attributes \
  --queue-url http://localhost:4566/000000000000/extraction-queue-dlq \
  --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

awslocal sqs create-queue --queue-name extraction-queue \
  --attributes "{\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"3\\\"}\"}"

awslocal dynamodb create-table \
  --table-name ChatbotDocuments \
  --attribute-definitions AttributeName=id,AttributeType=S AttributeName=content_hash,AttributeType=S \
  --key-schema AttributeName=id,KeyType=HASH \
  --global-secondary-indexes '[{
    "IndexName": "content_hash-index",
    "KeySchema": [{"AttributeName":"content_hash","KeyType":"HASH"}],
    "Projection": {"ProjectionType":"ALL"},
    "ProvisionedThroughput": {"ReadCapacityUnits":5,"WriteCapacityUnits":5}
  }]' \
  --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5

awslocal dynamodb create-table \
  --table-name ChatbotChunks \
  --attribute-definitions AttributeName=id,AttributeType=S \
  --key-schema AttributeName=id,KeyType=HASH \
  --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5

awslocal dynamodb create-table \
  --table-name ChatbotSessions \
  --attribute-definitions AttributeName=session_id,AttributeType=S \
  --key-schema AttributeName=session_id,KeyType=HASH \
  --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5

awslocal dynamodb update-time-to-live \
  --table-name ChatbotSessions \
  --time-to-live-specification "Enabled=true, AttributeName=expires_at"

echo "LocalStack inicializado: bucket, filas (com DLQ) e tabelas criadas."
