/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/newt/newtutil"
	"mynewt.apache.org/newt/newt/pkg"
	"mynewt.apache.org/newt/newt/project"
	"mynewt.apache.org/newt/newt/target"
	"mynewt.apache.org/newt/util"
)

const TARGET_KEYWORD_ALL string = "all"
const TARGET_DEFAULT_DIR string = "targets"
const MFG_DEFAULT_DIR string = "mfgs"

func NewtUsage(cmd *cobra.Command, err error) {
	if err != nil {
		sErr := err.(*util.NewtError)
		log.Debugf("%s", sErr.StackTrace)
		fmt.Fprintf(os.Stderr, "Error: %s\n", sErr.Text)
	}

	if cmd != nil {
		fmt.Printf("\n")
		fmt.Printf("%s - ", cmd.Name())
		cmd.Help()
	}
	os.Exit(1)
}

// Display help text with a max line width of 79 characters
func FormatHelp(text string) string {
	// first compress all new lines and extra spaces
	words := regexp.MustCompile("\\s+").Split(text, -1)
	linelen := 0
	fmtText := ""
	for _, word := range words {
		word = strings.Trim(word, "\n ") + " "
		tmplen := linelen + len(word)
		if tmplen >= 80 {
			fmtText += "\n"
			linelen = 0
		}
		fmtText += word
		linelen += len(word)
	}
	return fmtText
}

func ResolveTarget(name string) *target.Target {
	// Trim trailing slash from name.  This is necessary when tab
	// completion is used to specify the name.
	name = strings.TrimSuffix(name, "/")

	targetMap := target.GetTargets()

	// Check for fully-qualified name.
	if t := targetMap[name]; t != nil {
		return t
	}

	// Check the local "targets" directory.
	if t := targetMap[TARGET_DEFAULT_DIR+"/"+name]; t != nil {
		return t
	}

	// Check each repo alphabetically.
	fullNames := []string{}
	for fullName, _ := range targetMap {
		fullNames = append(fullNames, fullName)
	}
	for _, fullName := range util.SortFields(fullNames...) {
		if name == filepath.Base(fullName) {
			return targetMap[fullName]
		}
	}

	return nil
}

// Resolves a list of target names and checks for the optional "all" keyword
// among them.  Regardless of whether "all" is specified, all target names must
// be valid, or an error is reported.
//
// @return                      targets, all (t/f), err
func ResolveTargetsOrAll(names ...string) ([]*target.Target, bool, error) {
	targets := []*target.Target{}
	all := false

	for _, name := range names {
		if name == "all" {
			all = true
		} else {
			t := ResolveTarget(name)
			if t == nil {
				return nil, false,
					util.NewNewtError("Could not resolve target name: " + name)
			}

			targets = append(targets, t)
		}
	}

	return targets, all, nil
}

func ResolveTargets(names ...string) ([]*target.Target, error) {
	targets, all, err := ResolveTargetsOrAll(names...)
	if err != nil {
		return nil, err
	}
	if all {
		return nil,
			util.NewNewtError("Keyword \"all\" not allowed in thie context")
	}

	return targets, nil
}

func ResolveNewTargetName(name string) (string, error) {
	repoName, pkgName, err := newtutil.ParsePackageString(name)
	if err != nil {
		return "", err
	}

	if repoName != "" {
		return "", util.NewNewtError("Target name cannot contain repo; " +
			"must be local")
	}

	if pkgName == TARGET_KEYWORD_ALL {
		return "", util.NewNewtError("Target name " + TARGET_KEYWORD_ALL +
			" is reserved")
	}

	// "Naked" target names translate to "targets/<name>".
	if !strings.Contains(pkgName, "/") {
		pkgName = TARGET_DEFAULT_DIR + "/" + pkgName
	}

	if target.GetTargets()[pkgName] != nil {
		return "", util.NewNewtError("Target already exists: " + pkgName)
	}

	return pkgName, nil
}

func PackageNameList(pkgs []*pkg.LocalPackage) string {
	var buffer bytes.Buffer
	for i, pack := range pkgs {
		if i != 0 {
			buffer.WriteString(" ")
		}
		buffer.WriteString(pack.Name())
	}

	return buffer.String()
}

func ResetGlobalState() error {
	// Make sure the current working directory is at the project base.
	if err := os.Chdir(project.GetProject().Path()); err != nil {
		return util.NewNewtError("Failed to reset global state: " +
			err.Error())
	}

	target.ResetTargets()
	project.ResetProject()

	return nil
}

func InitProject() *project.Project {
	var p *project.Project
	var err error

	if p, err = project.TryGetProject(); err != nil {
		NewtUsage(nil, err)
	}

	for _, w := range p.Warnings() {
		util.ErrorMessage(util.VERBOSITY_QUIET, "* Warning: %s\n", w)
	}

	return p
}
