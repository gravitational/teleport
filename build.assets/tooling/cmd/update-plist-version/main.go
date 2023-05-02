// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command update-plist-version updates the version fields of a bundle plist.
//
// The version fields updated in an Info.plist file are the CFBundleVersion and
// CFBundleShortVersionString fields. If the version is not valid as per the
// Apple specification then the fields will be set to 1.0. A valid version is 3
// positive integers separated by dots. A semver with a pre-release tag is not
// valid.
//
// This is intended to be used on the tsh.app Info.plist files. Standard
// releases have the required version number. Pre-releases do not.

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"howett.net/plist"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <version> <plist-file>...\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	// A version can only be three positive integers separated by periods.
	versionRE := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	version := os.Args[1]
	if !versionRE.MatchString(version) {
		version = "1.0"
	}

	for _, filename := range os.Args[2:] {
		err := replaceFile(filename, func(in io.ReadSeeker, out io.Writer) error {
			return updateVersion(version, in, out)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not update version of %s: %v\n", filename, err)
			os.Exit(1)
		}
	}
}

func updateVersion(version string, in io.ReadSeeker, out io.Writer) error {
	dict := map[string]any{}
	d := plist.NewDecoder(in)
	if err := d.Decode(&dict); err != nil {
		return err
	}

	if _, ok := dict["CFBundleVersion"]; !ok {
		return errors.New("CFBundleVersion not in plist file")
	}
	if _, ok := dict["CFBundleShortVersionString"]; !ok {
		return errors.New("CFBundleShortVersionString not in plist file")
	}
	dict["CFBundleVersion"] = version
	dict["CFBundleShortVersionString"] = version

	e := plist.NewEncoder(out)
	e.Indent("\t")
	if err := e.Encode(dict); err != nil {
		return err
	}
	// the plist encoder does not write a newline at the end of the last line.
	_, err := out.Write([]byte{'\n'})
	return err
}

// replaceFile calls fn to replace the contents of filename. fn is passed the
// input file and the output file and should return when it has finished
// writing to out or an error occurs. The file will be replaced atomically on
// success, or will be left untouched on error. File permission of filename is
// retained, ownership and attributes are not.
func replaceFile(filename string, fn func(in io.ReadSeeker, out io.Writer) error) (rerr error) {
	inf, err := os.Open(filename)
	if err != nil {
		return err
	}
	// defer inf.Close() and only return the error if not returning another error.
	defer func() {
		if err := inf.Close(); err != nil && rerr == nil {
			rerr = err
		}
	}()

	outdir := filepath.Dir(filename)
	outname := filepath.Base(filename)
	// The temp file needs to be created in "outdir" so we can atomically rename
	// it to the input filename when we're successfully done.
	outf, err := os.CreateTemp(outdir, outname)
	if err != nil {
		return err
	}
	defer func() {
		// If we're returning an error we need to delete the temp file
		if rerr != nil {
			// Print any os.Remove() error on stderr as we are already returning an error
			if err := os.Remove(outf.Name()); err != nil {
				fmt.Fprintf(os.Stderr, "Could not remove temp file %s: %v\n", outf.Name(), err)
			}
		}
		if err := outf.Close(); err != nil && rerr == nil {
			rerr = err
		}
	}()

	fi, err := inf.Stat()
	if err != nil {
		return err
	}
	if err := outf.Chmod(fi.Mode() & fs.ModePerm); err != nil {
		return err
	}
	if err := fn(inf, outf); err != nil {
		return err
	}

	// os.Rename() must be the last thing this function does for the previous
	// defer cleanup to work properly.
	return os.Rename(outf.Name(), filename)
}
