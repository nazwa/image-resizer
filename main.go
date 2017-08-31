package main

import (
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go"
	"github.com/olebedev/config"
	"github.com/sqs/s3"
	"github.com/sqs/s3/s3util"
	"github.com/stvp/rollbar"
	"github.com/yvasiyarov/gorelic"
	"sourcegraph.com/sourcegraph/s3cache"
)

var (
	gorelicAgent *gorelic.Agent
	S3           *minio.Client
	cfg          *config.Config
)

// New returns a new Cache with underlying storage in Amazon S3. The bucketURL
// is the full URL to the bucket on Amazon S3, including the bucket name and AWS
// region (e.g., "https://s3-us-west-2.amazonaws.com/mybucket").
//
// The environment variables AWS_ACCESS_KEY_ID and AWS_SECRET_KEY are used as the AWS
// credentials. To use different credentials, modify the returned Cache object
// or construct a Cache object manually.
func NewS3Cache(bucketURL string) *s3cache.Cache {
	return &s3cache.Cache{
		Config: s3util.Config{
			Keys: &s3.Keys{
				AccessKey: cfg.UString("services.s3.accessKey"),
				SecretKey: cfg.UString("services.s3.secretKey"),
			},
			Service: s3.DefaultService,
		},
		BucketURL: bucketURL,
	}
}

func main() {
	// Parse config yaml string from ./conf.go
	var err error
	cfg, err = config.ParseYaml(confString)
	if err != nil {
		panic(err)
	}
	cfg.Env()

	rollbar.Token = cfg.UString("services.rollbar.token")
	rollbar.Environment = cfg.UString("services.appName")

	gorelicAgent = gorelic.NewAgent()
	gorelicAgent.Verbose = cfg.UBool("services.newRelic.verbose")
	gorelicAgent.NewrelicLicense = cfg.UString("services.newRelic.token")
	gorelicAgent.NewrelicName = cfg.UString("services.appName")

	if gorelicAgent.NewrelicLicense != "" {
		gorelicAgent.Run()
	}

	S3, err = minio.New(
		cfg.UString("services.s3.bucketUrl"),
		cfg.UString("services.s3.accessKey"),
		cfg.UString("services.s3.secretKey"),
		true,
	)
	if err != nil {
		panic(err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(loggerMiddleware())

	NewResizeHandler(r.Group("/"), S3)

	err = r.Run(":" + cfg.UString("port"))
	if err != nil {
		panic(err)
	}
}
