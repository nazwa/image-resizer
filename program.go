package main

import (
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
	"github.com/minio/minio-go"
	"github.com/sqs/s3"
	"github.com/sqs/s3/s3util"
	"github.com/stvp/rollbar"
	"github.com/yvasiyarov/gorelic"
	"sourcegraph.com/sourcegraph/s3cache"
)

// Program structures.
// Define Start and Stop methods.
type program struct {
	exit chan struct{}

	gorelicAgent *gorelic.Agent
	S3           *minio.Client
}

func (p *program) Start(s service.Service) error {
	if service.Interactive() {
		logger.Info("Running in terminal.")
	} else {
		logger.Info("Running under service manager.")
	}
	p.exit = make(chan struct{})

	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) init() {
	rollbar.Token = config.Services.Rollbar.Token
	rollbar.Environment = config.Services.AppName

	p.gorelicAgent = gorelic.NewAgent()
	p.gorelicAgent.Verbose = config.Services.NewRelic.Verbose
	p.gorelicAgent.NewrelicLicense = config.Services.NewRelic.Token
	p.gorelicAgent.NewrelicName = config.Services.AppName

	if config.Services.NewRelic.Token != "" {
		p.gorelicAgent.Run()
	}

	var err error
	s3Config := config.Services.S3
	p.S3, err = minio.New(s3Config.BucketUrl, s3Config.AccessKey, s3Config.SecretKey, true)
	if err != nil {
		logger.Error(err)
		panic(err)
	}
}

func (p *program) run() error {
	// Allow the main thread to finish
	// This prevents the service from being terminated
	runtime.Gosched()

	p.init()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(loggerMiddleware(logger))

	NewResizeHandler(r.Group("/"), p.S3)

	return r.Run(":" + config.Port)
}

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
				AccessKey: config.Services.S3.AccessKey,
				SecretKey: config.Services.S3.SecretKey,
			},
			Service: s3.DefaultService,
		},
		BucketURL: bucketURL,
	}
}

func (p *program) Stop(s service.Service) error {
	// Any work in Stop should be quick, usually a few seconds at most.
	logger.Info("I'm Stopping!")
	close(p.exit)
	return nil
}
