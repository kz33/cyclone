/*
Copyright 2016 caicloud authors. All rights reserved.

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

package stage_test

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/caicloud/cyclone/pkg/docker/mock"
	"github.com/golang/mock/gomock"

	"github.com/caicloud/cyclone/pkg/api"
	"github.com/caicloud/cyclone/pkg/docker"
	"github.com/caicloud/cyclone/pkg/osutil"
	"github.com/caicloud/cyclone/pkg/worker/cycloneserver"
	_ "github.com/caicloud/cyclone/pkg/worker/scm/provider"
	. "github.com/caicloud/cyclone/pkg/worker/stage"
)

const (
	logDir  = "/tmp/logs"
	codeDir = "/tmp/code"
)

var smTest StageManager
var dmTest *docker.DockerManager
var clientTest cycloneserver.CycloneServerClient
var eventTest *api.Event

func init() {
	// Log to standard error instead of files.
	flag.Set("logtostderr", "true")

	// Init the common clients.
	endpoint := "unix:///var/run/docker.sock"
	if runtime.GOOS == "darwin" {
		//	endpoint = "unix:////Users/zhujian/Library/Containers/com.docker.docker/Data/s60"
	}

	registry := api.Registry{
		Server:     "caicloud.test.com",
		Repository: "ptest",
		Username:   "",
		Password:   "",
	}

	eventTest = &api.Event{
		ID: "1",
		Project: &api.Project{
			Registry: &registry,
		},
		Pipeline: &api.Pipeline{},
		PipelineRecord: &api.PipelineRecord{
			PerformParams: &api.PipelinePerformParams{
				Ref: "refs/heads/master",
			},
			StageStatus: &api.StageStatus{},
		},
	}

	var err error
	dmTest, err = docker.NewDockerManager(endpoint, registry.Server, registry.Username, registry.Password)
	if err != nil {
		panic(err)
	}

	clientTest = cycloneserver.NewFakeClient("http://fack-server.cyclone.com")
	smTest = NewStageManager(dmTest, clientTest, eventTest.Project.Registry, eventTest.PipelineRecord.PerformParams)
	smTest.SetEvent(eventTest)

	fmt.Println("Initialization of common clients has finished")
}

func TestExecCodeCheckout(t *testing.T) {
	testCases := map[string]struct {
		inputs *api.CodeCheckoutStage
		pass   bool
	}{
		"correct public github": {
			&api.CodeCheckoutStage{
				MainRepo: &api.CodeSource{
					Type: api.Github,
					Github: &api.GitSource{
						Url: "https://github.com/caicloud/toy-dockerfile.git",
					},
				},
				DepRepos: []*api.DepRepo{
					&api.DepRepo{
						CodeSource: api.CodeSource{
							Type: api.Github,
							Github: &api.GitSource{
								Url: "https://github.com/caicloud/toy-dockerfile.git",
							},
						},
						Folder: "dep",
					},
				},
			},
			true,
		},
		//"correct svn": {
		//	&api.CodeCheckoutStage{
		//		MainRepo: &api.CodeSource{
		//			Type: api.SVN,
		//			SVN: &api.GitSource{
		//				Url: "http://192.168.21.100/svn/caicloud/cyclone",
		//			},
		//		},
		//	},
		//	true,
		//},
		//"correct private github": {
		//	&api.CodeCheckoutStage{
		//		MainRepo: &api.CodeSource{
		//			Type: api.Github,
		//			Github: &api.GitSource{
		//				Url: "https://github.com/caicloud/dockerfile.git",
		//			},
		//		},
		//	},
		//	false,
		//},
		//"wrong github": {
		//	&api.CodeCheckoutStage{
		//		MainRepo: &api.CodeSource{
		//			Type: api.Github,
		//			Github: &api.GitSource{
		//				Url: "https://github.com/caicloud/abc.git",
		//			},
		//		},
		//	},
		//	false,
		//},
	}

	stage := &api.CodeCheckoutStage{}
	for d, tc := range testCases {
		// Cleanup the temp folder.
		os.RemoveAll(codeDir)
		stage = tc.inputs
		err := smTest.ExecCodeCheckout("", stage)
		if tc.pass && err != nil || !tc.pass && err == nil {
			t.Errorf("%s failed as error: %v", d, err)
		}
	}
}

func TestExecImageBuild(t *testing.T) {
	// Prepare the code folder.
	dockerfileContent := "FROM alpine \nADD README.md /README.md"
	readmeContent := "Hello Cyclone"
	os.RemoveAll(codeDir)
	os.MkdirAll(codeDir, os.ModePerm)
	osutil.ReplaceFile(codeDir+"/Dockerfile", strings.NewReader(dockerfileContent))
	osutil.ReplaceFile(codeDir+"/README.md", strings.NewReader(readmeContent))

	testCases := map[string]struct {
		inputs []*api.ImageBuildInfo
		pass   bool
	}{
		"default Dockerfile": {
			[]*api.ImageBuildInfo{
				&api.ImageBuildInfo{
					TaskName:  "test1",
					ImageName: "cargo.caicloud.io/caicloud/test:v1",
				},
			},
			true,
		},
		"correct Dockerfile path": {
			[]*api.ImageBuildInfo{
				&api.ImageBuildInfo{
					ImageName:      "cargo.caicloud.io/caicloud/test:v1",
					DockerfilePath: "Dockerfile",
				},
			},
			true,
		},
		"wrong Dockerfile path": {
			[]*api.ImageBuildInfo{
				&api.ImageBuildInfo{
					ImageName:      "cargo.caicloud.io/caicloud/test:v1",
					DockerfilePath: "Dockerfile-abc",
				},
			},
			false,
		},
		"correct Dockerfile content": {
			[]*api.ImageBuildInfo{
				&api.ImageBuildInfo{
					ImageName:  "cargo.caicloud.io/caicloud/test:v1",
					Dockerfile: "FROM alpine \nADD README.md /README.md",
				},
			},
			true,
		},
		"multiple image build": {
			[]*api.ImageBuildInfo{
				&api.ImageBuildInfo{
					TaskName:       "v1",
					ImageName:      "cargo.caicloud.io/caicloud/test:v1",
					DockerfilePath: "Dockerfile",
				},
				&api.ImageBuildInfo{
					TaskName:       "v2",
					ImageName:      "cargo.caicloud.io/caicloud/test:v2",
					DockerfilePath: "Dockerfile",
				},
			},
			true,
		},
		// "wrong Dockerfile content": {
		// 	[]*api.ImageBuildInfo{
		// 		&api.ImageBuildInfo{
		// 			ImageName:  "cargo.caicloud.io/caicloud/test:v1",
		// 			Dockerfile: "FROM alpine \nADD readme.md /readme.md",
		// 		},
		// 	},
		// 	false,
		// },
	}

	stage := &api.ImageBuildStage{}
	for d, tc := range testCases {
		stage.BuildInfos = tc.inputs
		_, err := smTest.ExecImageBuild(stage)
		if tc.pass && err != nil || !tc.pass && err == nil {
			t.Errorf("%s failed as error: %v", d, err)
		}
	}
}

func TestExecPackage(t *testing.T) {
	// Prepare the code folder.
	dockerfileContent := "FROM alpine \nADD README.md /README.md"
	readmeContent := "Hello Cyclone"
	os.RemoveAll(codeDir)
	os.MkdirAll(codeDir, os.ModePerm)
	osutil.ReplaceFile(codeDir+"/Dockerfile", strings.NewReader(dockerfileContent))
	osutil.ReplaceFile(codeDir+"/README.md", strings.NewReader(readmeContent))

	testCases := map[string]struct {
		buildImage    *api.BuilderImage
		buildInfo     *api.BuildInfo
		unitTestStage *api.UnitTestStage
		packageStage  *api.PackageStage
		pass          bool
	}{
		"correct": {
			buildImage: &api.BuilderImage{
				Image: "busybox",
				EnvVars: []api.EnvVar{
					api.EnvVar{
						Name:  "TEST",
						Value: "TEST",
					},
				},
			},
			buildInfo: &api.BuildInfo{
				BuildTool: &api.BuildTool{
					Name:    api.MavenBuildTool,
					Version: "1.0",
				},

				CacheDependency: true,
			},
			unitTestStage: &api.UnitTestStage{
				GeneralStage: api.GeneralStage{
					Command: []string{"ls -la"},
				},
				Outputs: []string{"README.md"},
			},
			packageStage: &api.PackageStage{
				GeneralStage: api.GeneralStage{
					Command: []string{"ls -la"},
				},
				Outputs: []string{"README.md"},
			},
			pass: true,
		},
	}

	for d, tc := range testCases {
		err := smTest.ExecPackage(tc.buildImage, tc.buildInfo, tc.unitTestStage, tc.packageStage)
		if tc.pass && err != nil || !tc.pass && err == nil {
			t.Errorf("%s failed as error: %v", d, err)
		}
	}
}

func TestExecIntegrationTest(t *testing.T) {
	// Prepare the code folder.
	dockerfileContent := "FROM alpine \nADD README.md /README.md"
	readmeContent := "Hello Cyclone"
	os.RemoveAll(codeDir)
	os.MkdirAll(codeDir, os.ModePerm)
	osutil.ReplaceFile(codeDir+"/Dockerfile", strings.NewReader(dockerfileContent))
	osutil.ReplaceFile(codeDir+"/README.md", strings.NewReader(readmeContent))

	testCases := map[string]struct {
		builtImages []string
		stage       *api.IntegrationTestStage
		pass        bool
	}{
		"correct": {
			builtImages: []string{"busybox:1.24.0"},
			stage: &api.IntegrationTestStage{
				Config: &api.IntegrationTestConfig{
					ImageName: "busybox:1.24.0",
					Command:   []string{"ls", "cd /"},
					EnvVars: []api.EnvVar{
						api.EnvVar{
							Name:  "TEST",
							Value: "TEST",
						},
					},
				},
				Services: []api.Service{
					api.Service{
						Name:    "testService",
						Image:   "mongo:3.0.5",
						Command: []string{"mongod --smallfiles"},
						EnvVars: []api.EnvVar{
							api.EnvVar{
								Name:  "TEST",
								Value: "TEST",
							},
						},
					},
				},
			},
			pass: true,
		},
	}

	for d, tc := range testCases {
		err := smTest.ExecIntegrationTest(tc.builtImages, tc.stage)
		if tc.pass && err != nil || !tc.pass && err == nil {
			t.Errorf("%s failed as error: %v", d, err)
		}
	}
}

func TestExecImageRelease(t *testing.T) {
	// Log to standard error instead of files.
	flag.Set("logtostderr", "true")

	ctl := gomock.NewController(t)
	clientMock := mock_docker.NewMockClientInterface(ctl)
	clientMock.EXPECT().PushImage(gomock.Any(), gomock.Any()).Return(nil)

	dmTest.Client = clientMock

	smMock := NewStageManager(dmTest, clientTest, eventTest.Project.Registry, eventTest.PipelineRecord.PerformParams)
	smMock.SetEvent(eventTest)

	testCases := map[string]struct {
		builtImages []string
		stage       *api.ImageReleaseStage
		pass        bool
	}{
		"correct": {
			builtImages: []string{"busybox:1.24.0"},
			stage: &api.ImageReleaseStage{
				ReleasePolicies: []api.ImageReleasePolicy{
					api.ImageReleasePolicy{
						ImageName: "busybox:1.24.0",
						Type:      api.AlwaysRelease,
					},
				},
			},
			pass: true,
		},
	}

	for d, tc := range testCases {
		err := smMock.ExecImageRelease(tc.builtImages, tc.stage)
		if tc.pass && err != nil || !tc.pass && err == nil {
			t.Errorf("%s failed as error: %v", d, err)
		}
	}
}
