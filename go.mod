module github.com/example/test-go

go 1.22

require (
        github.com/aws/aws-lambda-go v0.0.0
        github.com/go-sql-driver/mysql v0.0.0
)

replace github.com/aws/aws-lambda-go => ./third_party/github.com/aws/aws-lambda-go

replace github.com/go-sql-driver/mysql => ./third_party/github.com/go-sql-driver/mysql
