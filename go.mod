module github.com/danubiobwm/chatbot-rag-go

go 1.22

require (
	github.com/aws/aws-sdk-go-v2 v1.30.3
	github.com/aws/aws-sdk-go-v2/config v1.27.27
	github.com/aws/aws-sdk-go-v2/credentials v1.17.27
	github.com/aws/aws-sdk-go-v2/service/bedrockruntime v1.13.4
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.33.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.58.3
	github.com/aws/aws-sdk-go-v2/service/sqs v1.34.4
	github.com/aws/aws-sdk-go-v2/service/textract v1.27.4
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.9.0
	go.uber.org/zap v1.27.0
)
