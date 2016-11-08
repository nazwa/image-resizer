package main

import (
	"github.com/kardianos/service"
)

// Config represents the configuration information.
type ConfigStruct struct {
	Debug    bool
	Port     string
	Root     string `json:"-"`
	Services struct {
		AppName string
		Rollbar struct {
			Token string
		}
		S3 struct {
			AccessKey string
			SecretKey string
			BucketUrl string
		}
		NewRelic struct {
			Token   string
			Verbose bool
		}
	}
	Service service.Config
	Resizer struct {
		BaseUrl string
		ScaleUp bool
		Cache   string
	}
}

var config = ConfigStruct{}
