// +build perf

package dynamodb

import (
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type noopReadCloser struct{}

func (r *noopReadCloser) Read(b []byte) (int, error) {
	return 0, io.EOF
}
func (r *noopReadCloser) Close() error {
	return nil
}

var noopBody = &noopReadCloser{}

func BenchmarkPutItem(b *testing.B) {
	cfg := aws.Config{
		Region:      aws.String("us-east-1"),
		DisableSSL:  aws.Bool(true),
		Credentials: credentials.NewStaticCredentials("AKID", "SECRET", ""),
	}
	server := successRespServer([]byte(`{}`))
	cfg.Endpoint = aws.String(server.URL)

	svc := dynamodb.New(&cfg)
	svc.Handlers.Send.Clear()
	svc.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       noopBody,
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		av, err := dynamodbattribute.ConvertToMap(dbItem{Key: "MyKey", Data: "MyData"})
		if err != nil {
			b.Fatal("benchPutItem, expect no ConvertToMap errors", err)
		}
		params := &dynamodb.PutItemInput{
			Item:      av,
			TableName: aws.String("tablename"),
		}
		_, err = svc.PutItem(params)
		if err != nil {
			b.Error("benchPutItem, expect no request errors", err)
		}
	}
}
