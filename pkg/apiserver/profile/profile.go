// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package profile

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

// the default time interval of profiling.
const grabInterval = 30

func fetchSvg(ctx context.Context, t, addr, filePrefix string) (string, error) {
	url := getURL(t, addr)
	if url == "" {
		return "", errors.Errorf("no such component: %s", t)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)
	if t == pd {
		// Forbidden PD follower proxy
		req.Header.Add("PD-Allow-follower-handle", "true")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("request %s failed: %s", url, resp.Status)
	}

	svgFilePath, err := getSvgFilePath(t, filePrefix, resp.Body)
	if err != nil {
		return "", err
	}
	return svgFilePath, nil
}

func getURL(t, addr string) string {
	var url string
	interval := fmt.Sprintf("%d", grabInterval)
	switch t {
	case pd:
		url = "/pd/api/v1/debug/pprof/profile?seconds=" + interval
	case tikv, tidb:
		url = "/debug/pprof/profile?seconds=" + interval
	default:
		return ""
	}
	return fmt.Sprintf("http://%s%s", addr, url)
}

func getSvgFilePath(t, filePrefix string, body io.ReadCloser) (string, error) {
	if t == tikv {
		svgFile, err := ioutil.TempFile("", filePrefix)
		if err != nil {
			return "", err
		}

		// Write the body to .svg file
		_, err = io.Copy(svgFile, body)
		if err != nil {
			return "", err
		}
		return svgFile.Name(), nil
	}
	tmpfile, err := ioutil.TempFile("", filePrefix)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// Write the body to temp file
	_, err = io.Copy(tmpfile, body)
	if err != nil {
		return "", err
	}
	svgFilePath := tmpfile.Name() + ".svg"
	if _, err := exec.Command(goCmd(), "tool", "pprof", "-svg", "-output", svgFilePath, tmpfile.Name()).CombinedOutput(); err != nil { //nolint:gosec
		return "", err
	}
	return svgFilePath, nil
}
