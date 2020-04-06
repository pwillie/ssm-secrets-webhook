// Copyright © 2020 Peter Wilson
//
// Copyright © 2020 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// This file has been repurposed for simple AWS SSM environment injection.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"emperror.dev/errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

const (
	ec2MetaDataServiceURL = "http://169.254.169.254/latest/dynamic/instance-identity/document"
)

type sanitizedEnviron []string

func (environ *sanitizedEnviron) append(name string, value string) {
	*environ = append(*environ, fmt.Sprintf("%s=%s", name, value))
}

type secretInjectorFunc func(key, value string)

func injectSecretsFromSsm(references map[string]string, inject secretInjectorFunc, ignoreMissingSecrets bool, logger logrus.FieldLogger) error {
	secretCache := make(map[string]*ssm.GetParameterOutput)

	// Create AWS client service
	region, err := getCurrentAwsRegion(logger)
	if err != nil {
		logger.Fatalf("failed to get current AWS region: %s", err)
	}
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String(region)},
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		logger.Fatal("failed to create SSM client", err.Error())
	}

	ssmsvc := ssm.New(sess, aws.NewConfig().WithRegion(region))

	for name, value := range references {
		if !strings.HasPrefix(value, "ssm:") {
			inject(name, value)
			continue
		}

		valuePath := strings.TrimPrefix(value, "ssm:")

		var secret *ssm.GetParameterOutput
		var err error

		if secret = secretCache[valuePath]; secret == nil {
			withDecryption := true
			secret, err = ssmsvc.GetParameter(&ssm.GetParameterInput{
				Name:           aws.String(valuePath),
				WithDecryption: aws.Bool(withDecryption),
			})
			if err != nil {
				if !ignoreMissingSecrets {
					return errors.WrapWithDetails(err, "failed to read secret from path:", valuePath)
				}
				logger.Errorln("failed to read secret from path:", valuePath, err.Error())
			} else {
				secretCache[valuePath] = secret
			}
		}

		if secret == nil {
			if !ignoreMissingSecrets {
				return errors.NewWithDetails("path not found:", valuePath)
			}

			logger.Errorln("path not found:", valuePath)
			continue
		}

		inject(name, *secret.Parameter.Value)
	}

	return nil
}

func getCurrentAwsRegion(logger logrus.FieldLogger) (string, error) {
	region, present := os.LookupEnv("AWS_REGION")

	if !present {
		logger.Infof("fetching %s", ec2MetaDataServiceURL)
		res, err := http.Get(ec2MetaDataServiceURL)
		if err != nil {
			return "", fmt.Errorf("Error fetching %s", ec2MetaDataServiceURL)
		}

		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("Error parsing %s", ec2MetaDataServiceURL)
		}

		var unmarshalled = map[string]string{}
		err = json.Unmarshal(body, &unmarshalled)
		if err != nil {
			logger.Warnf("Error unmarshalling %s, skip...\n", ec2MetaDataServiceURL)
		}

		region = unmarshalled["region"]
	}

	return region, nil
}

func main() {
	enableJSONLog := cast.ToBool(os.Getenv("SSM_JSON_LOG"))

	var logger logrus.FieldLogger
	{
		log := logrus.New()
		if enableJSONLog {
			log.SetFormatter(&logrus.JSONFormatter{})
		}
		logger = log.WithField("app", "ssm-env")
	}

	var entrypointCmd []string
	if len(os.Args) == 1 {
		logger.Fatalln("no command is given, ssm-env can't determine the entrypoint (command), please specify it explicitly or let the webhook query it (see documentation)")
	} else {
		entrypointCmd = os.Args[1:]
	}

	binary, err := exec.LookPath(entrypointCmd[0])
	if err != nil {
		logger.Fatalln("binary not found", entrypointCmd[0])
	}

	// Used both for reading secrets and transit encryption
	ignoreMissingSecrets := cast.ToBool(os.Getenv("SSM_IGNORE_MISSING_SECRETS"))

	// initial and sanitized environs
	environ := make(map[string]string, len(os.Environ()))
	sanitized := make(sanitizedEnviron, 0, len(environ))

	for _, env := range os.Environ() {
		split := strings.SplitN(env, "=", 2)
		name := split[0]
		value := split[1]
		environ[name] = value
	}

	inject := func(key, value string) {
		sanitized.append(key, value)
	}

	err = injectSecretsFromSsm(environ, inject, ignoreMissingSecrets, logger)
	if err != nil {
		logger.Fatalln("failed to inject secrets from ssm:", err)
	}

	logger.Infoln("spawning process:", entrypointCmd)

	err = syscall.Exec(binary, entrypointCmd, sanitized)
	if err != nil {
		logger.Fatalln("failed to exec process", entrypointCmd, err.Error())
	}
}
