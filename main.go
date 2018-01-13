package main

import (
	"io/ioutil"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/kardianos/osext"
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

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// Parse config yaml string from ./conf.go
	var err error
	cfg, err = config.ParseYaml(confString)
	if err != nil {
		panic(err)
	}

	// Dir is where the app was started. Handy
	root, _ := osext.ExecutableFolder()

	// We might have a file config
	configFilePath := filepath.Join(root, "config.json")

	// See if we have a file config, and if so - load it
	if fileBytes, err := ioutil.ReadFile(configFilePath); err == nil {
		// Parse the loaded file
		fileConf, err := config.ParseJson(string(fileBytes))
		Must(err)

		// Extend current config with new values
		cfg, err = cfg.Extend(fileConf)
		Must(err)
	}

	// Load from env
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
	Must(err)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(loggerMiddleware())

	NewResizeHandler(r.Group("/"), S3)

	Must(r.Run(":" + cfg.UString("port")))

}
