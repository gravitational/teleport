/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func main() {
	account := kingpin.Flag("aws-account", "AWS Account ID").Short('a').Required().String()
	regions := kingpin.Flag("regions", "List of AWS regions to get and update AMIs for").Short('r').Required().String()
	amiType := kingpin.Flag("type", "Type of AMI: 'oss', 'ent', or 'ent-fips'").Short('t').Required().Enum(string(OSS), string(Ent), string(FIPS))
	version := kingpin.Flag("version", "Teleport version to update AMIs with").Short('v').Required().String()
	kingpin.Parse()

	ctx := context.Background()

	imageIDs := make(map[string]string)

	for _, region := range strings.Split(*regions, ",") {
		stub := fmt.Sprintf("gravitational-teleport-ami-%v-%v", *amiType, *version)
		if *amiType == "ent-fips" {
			stub = fmt.Sprintf("gravitational-teleport-ami-ent-%v-fips", *version)
		}

		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			log.Fatalf("could not load AWS config: %v", err)
		}

		client := ec2.NewFromConfig(cfg)
		resp, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{
			Filters: []types.Filter{
				{Name: aws.String("name"), Values: []string{stub}},
				{Name: aws.String("is-public"), Values: []string{"true"}},
			},
			Owners: []string{*account},
		})
		if err != nil {
			log.Fatalf("describe images: %v", err)
		}

		if l := len(resp.Images); l != 1 {
			log.Fatalf("expected 1 image for %v, got %v", stub, l)
		}

		id := resp.Images[0].ImageId
		if id == nil {
			log.Fatalf("image %v is missing ID", stub)
		}
		imageIDs[region] = *id
	}

	tfDir := filepath.Join("..", "..", "examples", "aws", "terraform")

	// get a list of non-hidden directories one level under terraform
	// (one for each mode)
	files, err := os.ReadDir(tfDir)
	if err != nil {
		log.Fatalf("could not read %v: %v", tfDir, err)
	}
	var tfModes []string
	for _, file := range files {
		if file.IsDir() && !strings.HasPrefix(file.Name(), ".") {
			tfModes = append(tfModes, file.Name())
		}
	}
	// change version in TF_VAR_ami_name strings
	for _, tfMode := range tfModes {
		log.Printf("Updating version in README for %v", tfMode)
		re, err := regexp.Compile(fmt.Sprintf(`gravitational-teleport-ami-%s-([0-9.]+)`, *amiType))
		if err != nil {
			log.Fatalf("invalid regexp for type %q: %v", *amiType, err)
		}

		readme := filepath.Join(tfDir, tfMode, "README.md")
		b, err := os.ReadFile(readme)
		if err != nil {
			log.Fatalf("could not find README.md for terraform mode %q: %v", tfMode, err)
		}

		replaced := re.ReplaceAll(b, []byte(fmt.Sprintf("gravitational-teleport-ami-%s-%s", *amiType, *version)))
		if err := os.WriteFile(readme, replaced, 0644); err != nil {
			log.Fatalf("could not update %v: %v", readme, err)
		}
	}
	// replace AMI ID in place
	tfPath := filepath.Join(tfDir, "AMIS.md")
	md, err := os.ReadFile(tfPath)
	if err != nil {
		log.Fatalf("could not read %v: %v", tfPath, err)
	}

	for _, region := range strings.Split(*regions, ",") {
		newAMI := imageIDs[region]

		ts := AMIType(*amiType)
		re, err := regexp.Compile(fmt.Sprintf(`(?m)^# %s v(.*) %s: (ami-.*)$`, region, ts.FriendlyType()))
		if err != nil {
			log.Fatalf("invalid regexp for region %q type %q: %v", region, *amiType, err)
		}

		repl := fmt.Sprintf(`# %s v%s %s: %s`, region, *version, ts.FriendlyType(), newAMI)
		md = re.ReplaceAll(md, []byte(repl))

		log.Printf("[%v: %v] -> %v", *amiType, region, newAMI)
	}
	if err := os.WriteFile(tfPath, md, 0644); err != nil {
		log.Fatalf("could not update %v: %v", tfPath, err)
	}
}

type AMIType string

const (
	OSS  AMIType = "oss"
	Ent  AMIType = "ent"
	FIPS AMIType = "ent-fips"
)

func (a AMIType) FriendlyType() string {
	switch a {
	case OSS:
		return "OSS"
	case Ent:
		return "Enterprise"
	case FIPS:
		return "Enterprise FIPS"
	default:
		return ""
	}
}
