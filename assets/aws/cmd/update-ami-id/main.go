/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

	imageIDs := make(map[string]map[string]string)

	for _, region := range strings.Split(*regions, ",") {
		for _, arch := range []string{"x86_64", "arm64"} {
			edition := *amiType
			fips := "false"
			if *amiType == "ent-fips" {
				edition = "ent"
				fips = "true"
			}

			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				log.Fatalf("could not load AWS config for %q: %v", region, err)
			}

			client := ec2.NewFromConfig(cfg)
			resp, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{
				Filters: []types.Filter{
					{Name: aws.String("name"), Values: []string{"teleport-*"}},
					{Name: aws.String("is-public"), Values: []string{"true"}},
					{Name: aws.String("tag:Architecture"), Values: []string{arch}},
					{Name: aws.String("tag:TeleportVersion"), Values: []string{*version}},
					{Name: aws.String("tag:TeleportEdition"), Values: []string{edition}},
					{Name: aws.String("tag:TeleportFipsEnabled"), Values: []string{fips}},
					{Name: aws.String("tag:BuildType"), Values: []string{"production"}},
				},
				Owners: []string{*account},
			})
			if err != nil {
				log.Fatalf("describe images in %q: %v", region, err)
			}

			if l := len(resp.Images); l != 1 {
				if l == 0 {
					log.Printf("missing image for region %q type %q ver %q arch %q", region, *amiType, *version, arch)
					continue
				}
				log.Fatalf("expected 1 image for region %q type %q ver %q arch %q, got %v", region, *amiType, *version, arch, l)
			}

			id := resp.Images[0].ImageId
			if id == nil {
				log.Fatalf("image for region %q type %q ver %q arch %q is missing ID", region, *amiType, *version, arch)
			}
			if _, ok := imageIDs[region]; !ok {
				imageIDs[region] = make(map[string]string)
			}
			imageIDs[region][arch] = *id
		}
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
		re, err := regexp.Compile(fmt.Sprintf(`teleport-%s-([0-9.]+)`, *amiType))
		if err != nil {
			log.Fatalf("invalid regexp for type %q: %v", *amiType, err)
		}

		readme := filepath.Join(tfDir, tfMode, "README.md")
		b, err := os.ReadFile(readme)
		if err != nil {
			log.Fatalf("could not find README.md for terraform mode %q: %v", tfMode, err)
		}

		replaced := re.ReplaceAll(b, []byte(fmt.Sprintf("teleport-%s-%s", *amiType, *version)))
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
		for _, arch := range []string{"x86_64", "arm64"} {
			newAMI := imageIDs[region][arch]
			if newAMI == "" {
				continue
			}

			ts := AMIType(*amiType)
			re, err := regexp.Compile(fmt.Sprintf(`(?m)^# %s v(.*) %s %s: (ami-.*)$`, region, arch, ts.FriendlyType()))
			if err != nil {
				log.Fatalf("invalid regexp for region %q type %q arch %q: %v", region, *amiType, arch, err)
			}

			repl := fmt.Sprintf(`# %s v%s %s %s: %s`, region, *version, arch, ts.FriendlyType(), newAMI)
			md = re.ReplaceAll(md, []byte(repl))

			log.Printf("[%v %v: %v] -> %v", *amiType, arch, region, newAMI)
		}
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
