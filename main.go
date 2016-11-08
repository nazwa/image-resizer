package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/kardianos/osext"
	"github.com/kardianos/service"
)

func loadJsonFile(fullPath string, target interface{}) {
	configFile, err := os.Open(fullPath)
	if err != nil {
		panic(err)
	}

	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(target); err != nil {
		panic(err)
	}
}

var logger service.Logger

// Service setup.
//   Define service config.
//   Create the service.
//   Setup the logger.
//   Handle service controls (optional).
//   Run the service.
func main() {
	root, _ := osext.ExecutableFolder()
	loadJsonFile(filepath.Join(root, "config.json"), &config)

	svcFlag := flag.String("service", "", "Control the system service.")
	flag.Parse()

	prg := &program{}
	s, err := service.New(prg, &config.Service)
	if err != nil {
		log.Fatal(err)
	}
	errs := make(chan error, 5)
	logger, err = s.Logger(errs)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			err := <-errs
			if err != nil {
				logger.Error(err)
			}
		}
	}()

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			logger.Infof("Valid actions: %q\n", service.ControlAction)
			logger.Error(err)
		}
		return
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
