//
//  This file is part of go-apt-client library
//
//  Copyright (C) 2017  Arduino AG (http://www.arduino.cc/)
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package apt

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Package is a package available in the APT system
type Package struct {
	Name             string
	Status           string
	Architecture     string
	Version          string
	ShortDescription string
	InstalledSizeKB  int
}

// List returns a list of packages available in the system with their
// respective status.
func List() ([]*Package, error) {
	return Search("*")
}

// Search list packages available in the system that match the search
// pattern
func Search(pattern string) ([]*Package, error) {
	cmd := exec.Command("dpkg-query", "-W", "-f=${Package}\t${Architecture}\t${db:Status-Status}\t${Version}\t${Installed-Size}\t${Binary:summary}\n", pattern)

	out, err := cmd.CombinedOutput()
	if err != nil {
		// Avoid returning an error if the list is empty
		if bytes.Contains(out, []byte("no packages found matching")) {
			return []*Package{}, nil
		}
		return nil, fmt.Errorf("running dpkg-query: %s - %s", err, out)
	}

	return parseDpkgQueryOutput(out), nil
}

func parseDpkgQueryOutput(out []byte) []*Package {
	res := []*Package{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		data := strings.Split(scanner.Text(), "\t")
		size, err := strconv.Atoi(data[4])
		if err != nil {
			// Ignore error
			size = 0
		}
		res = append(res, &Package{
			Name:             data[0],
			Architecture:     data[1],
			Status:           data[2],
			Version:          data[3],
			InstalledSizeKB:  size,
			ShortDescription: data[5],
		})
	}
	return res
}

// CheckForUpdates runs an apt update to retrieve new packages available
// from the repositories
func CheckForUpdates() (output []byte, err error) {
	cmd := exec.Command("apt-get", "update", "-q")
	return cmd.CombinedOutput()
}

// ListUpgradable return all the upgradable packages and the version that
// is going to be installed if an UpgradeAll is performed
func ListUpgradable() ([]*Package, error) {
	cmd := exec.Command("apt", "list", "--upgradable")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running apt list: %s", err)
	}
	re := regexp.MustCompile(`^([^ ]+) ([^ ]+) ([^ ]+)( \[upgradable from: [^\[\]]*\])?`)

	res := []*Package{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		matches := re.FindAllStringSubmatch(scanner.Text(), -1)
		if len(matches) == 0 {
			continue
		}

		// Remove repository information in name
		// example: "libgweather-common/zesty-updates,zesty-updates"
		//       -> "libgweather-common"
		name := strings.Split(matches[0][1], "/")[0]

		res = append(res, &Package{
			Name:         name,
			Status:       "upgradable",
			Version:      matches[0][2],
			Architecture: matches[0][3],
		})
	}
	return res, nil
}

// Upgrade runs the upgrade for a set of packages
func Upgrade(packs ...*Package) (output []byte, err error) {
	args := []string{"upgrade", "-y"}
	for _, pack := range packs {
		if pack == nil || pack.Name == "" {
			return nil, fmt.Errorf("apt.Upgrade: Invalid package with empty Name")
		}
		args = append(args, pack.Name)
	}
	cmd := exec.Command("apt-get", args...)
	return cmd.CombinedOutput()
}

// UpgradeAll upgrade all upgradable packages
func UpgradeAll() (output []byte, err error) {
	cmd := exec.Command("apt-get", "upgrade", "-y")
	return cmd.CombinedOutput()
}

// DistUpgrade upgrades all upgradable packages, it may remove older versions to install newer ones.
func DistUpgrade() (output []byte, err error) {
	cmd := exec.Command("apt-get", "dist-upgrade", "-y")
	return cmd.CombinedOutput()
}

// Remove removes a set of packages
func Remove(packs ...*Package) (output []byte, err error) {
	args := []string{"remove", "-y"}
	for _, pack := range packs {
		if pack == nil || pack.Name == "" {
			return nil, fmt.Errorf("apt.Remove: Invalid package with empty Name")
		}
		args = append(args, pack.Name)
	}
	cmd := exec.Command("apt-get", args...)
	return cmd.CombinedOutput()
}

// Install installs a set of packages
func Install(packs ...*Package) (output []byte, err error) {
	args := []string{"install", "-y"}
	for _, pack := range packs {
		if pack == nil || pack.Name == "" {
			return nil, fmt.Errorf("apt.Install: Invalid package with empty Name")
		}
		args = append(args, pack.Name)
	}
	cmd := exec.Command("apt-get", args...)
	return cmd.CombinedOutput()
}

// Install tries to install a set of packages
func InstallDry(packs ...*Package) (output []byte, err error) {
	args := []string{"install", "-y", "--dry-run"}
	for _, pack := range packs {
		if pack == nil || pack.Name == "" {
			return nil, fmt.Errorf("apt.Install: Invalid package with empty Name")
		}
		args = append(args, pack.Name)
	}
	cmd := exec.Command("apt-get", args...)
	return cmd.CombinedOutput()
}

// Download triest to downlaod a set of packages
// targetPath should be absolute.
func Download(pack *Package, targetPath string) (output []byte, err error) {
	args := []string{"install", "-y", "--reinstall", "--download-only",
		"-o", "Debug::NoLocking=1",
		"-o", fmt.Sprintf("Dir::Cache::archives=\"%s\"", targetPath),
	}
	if pack == nil || pack.Name == "" {
		return nil, fmt.Errorf("apt.Download: Invalid package with empty Name")
	}
	// This is to resolve an issue with apt not always creating a "partial"
	// directory in the target cache directory. SMH.
	err = os.MkdirAll(path.Join(targetPath, "partial"), 0755)
	if err != nil {
		return nil, err
	}
	args = append(args, pack.Name)
	cmd := exec.Command("apt-get", args...)
	return cmd.CombinedOutput()
}

// Get a list of dependencies, from the bottom up.
func GetDependencies(pack *Package) (list []string, err error) {
	args := []string{"depends", "-i", "--recurse"}
	if pack == nil || pack.Name == "" {
		return nil, fmt.Errorf("apt.GetDependencies: Invalid package with empty Name")
	}
	args = append(args, pack.Name)
	cmd := exec.Command("apt-cache", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	stringOutput := string(out)
	// Obtains the output line-per-line.
	splitOutput := strings.Split(stringOutput, "\n")
	// Reverses it.
	slices.Reverse(splitOutput)

	depSeen := make(map[string]struct{})
	depList := []string{}
	for _, dep := range splitOutput {
		split := strings.Split(dep, " ")
		if len(split) > 0 {
			depName := split[len(split)-1]
			if depName == "" || strings.Compare(depName, pack.Name) == 0 {
				continue
			}
			if _, ok := depSeen[depName]; ok {
				continue // Skips to the next iteration
			}
			depSeen[depName] = struct{}{}
			depList = append(depList, depName)
		}
	}
	return depList, nil
}
