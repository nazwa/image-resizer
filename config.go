package main

var (
	// Debug var to switch mode from outside
	debug string
	// CommitHash exported to assign it from main.go
	commitHash string
)

// Most easiest way to configure
// an application is define config as
// yaml string and then parse it into
// map.
// How it works see here:
//     https://github.com/olebedev/config
var confString = `
debug: true
port: 80
title: Image Resizer
services:
  appName:
  rollbar:
    token:
  s3:
    accessKey:
    secretKey:
    endpoint:
  newRelic:
    token:
    verbose:
`
