// +build perf_live

package dynamodb

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const testPutItemCount = 5000
const testTableName = `perfTable`

func BenchmarkLivePutItem(b *testing.B) {
	benchPutItemParallel(1, testPutItemCount, b)
}

func BenchmarkLivePutItemParallel5(b *testing.B) {
	benchPutItemParallel(5, testPutItemCount, b)
}

func BenchmarkLivePutItemParallel10(b *testing.B) {
	benchPutItemParallel(10, testPutItemCount, b)
}

func BenchmarkLivePutItemParallel20(b *testing.B) {
	benchPutItemParallel(20, testPutItemCount, b)
}

func benchPutItemParallel(p, c int, b *testing.B) {
	svc := dynamodb.New(&aws.Config{
		DisableSSL: aws.Bool(true),
	})

	av, err := dynamodbattribute.ConvertToMap(dbItem{Key: "MyKey", Data: "MyData"})
	if err != nil {
		b.Fatal("expect no ConvertToMap errors", err)
	}
	params := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(testTableName),
	}
	b.N = c

	b.ResetTimer()
	b.SetParallelism(p)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err = svc.PutItem(params)
			if err != nil {
				b.Error("expect no request errors", err)
			}
		}
	})
}
