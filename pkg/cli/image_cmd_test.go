//go:build ui_base
// +build ui_base

package cli //nolint:testpackage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	godigest "github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/resty.v1"
	zotErrors "zotregistry.io/zot/errors"
	"zotregistry.io/zot/pkg/api"
	"zotregistry.io/zot/pkg/api/config"
	extconf "zotregistry.io/zot/pkg/extensions/config"
	"zotregistry.io/zot/pkg/test"
)

func TestSearchImageCmd(t *testing.T) {
	Convey("Test image help", t, func() {
		args := []string{"--help"}
		configPath := makeConfigFile("")
		defer os.Remove(configPath)
		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(buff.String(), ShouldContainSubstring, "Usage")
		So(err, ShouldBeNil)
		Convey("with the shorthand", func() {
			args[0] = "-h"
			configPath := makeConfigFile("")
			defer os.Remove(configPath)
			cmd := NewImageCommand(new(mockService))
			buff := bytes.NewBufferString("")
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err := cmd.Execute()
			So(buff.String(), ShouldContainSubstring, "Usage")
			So(err, ShouldBeNil)
		})
	})
	Convey("Test image no url", t, func() {
		args := []string{"imagetest", "--name", "dummyIdRandom"}
		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
		defer os.Remove(configPath)
		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
		So(err, ShouldEqual, zotErrors.ErrNoURLProvided)
	})

	Convey("Test image invalid home directory", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName"}

		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		err := os.Setenv("HOME", "nonExistentDirectory")
		if err != nil {
			panic(err)
		}

		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err = cmd.Execute()
		So(err, ShouldNotBeNil)

		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		err = os.Setenv("HOME", home)
		if err != nil {
			log.Fatal(err)
		}
	})

	Convey("Test image no params", t, func() {
		args := []string{"imagetest", "--url", "someUrl"}
		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
		defer os.Remove(configPath)
		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldBeNil)
	})

	Convey("Test image invalid url", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "--url", "invalidUrl"}
		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
		defer os.Remove(configPath)
		cmd := NewImageCommand(new(searchService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
		So(err, ShouldEqual, zotErrors.ErrInvalidURL)
		So(buff.String(), ShouldContainSubstring, "invalid URL format")
	})
	Convey("Test image invalid url port", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "--url", "http://localhost:99999"}
		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
		defer os.Remove(configPath)
		cmd := NewImageCommand(new(searchService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
		So(buff.String(), ShouldContainSubstring, "invalid port")

		Convey("without flags", func() {
			args := []string{"imagetest", "--url", "http://localhost:99999"}
			configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
			defer os.Remove(configPath)
			cmd := NewImageCommand(new(searchService))
			buff := bytes.NewBufferString("")
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err = cmd.Execute()
			So(err, ShouldNotBeNil)
			So(buff.String(), ShouldContainSubstring, "invalid port")
		})
	})
	Convey("Test image unreachable", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "--url", "http://localhost:9999"}
		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
		defer os.Remove(configPath)
		cmd := NewImageCommand(new(searchService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
	})

	Convey("Test image url from config", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName"}

		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		space := regexp.MustCompile(`\s+`)
		str := space.ReplaceAllString(buff.String(), " ")
		So(strings.TrimSpace(str), ShouldEqual, "IMAGE NAME TAG DIGEST SIZE dummyImageName tag DigestsA 123kB")
		So(err, ShouldBeNil)
	})

	Convey("Test image by name", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "--url", "someUrlImage"}
		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
		defer os.Remove(configPath)
		imageCmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		imageCmd.SetOut(buff)
		imageCmd.SetErr(buff)
		imageCmd.SetArgs(args)
		err := imageCmd.Execute()
		space := regexp.MustCompile(`\s+`)
		str := space.ReplaceAllString(buff.String(), " ")
		So(strings.TrimSpace(str), ShouldEqual, "IMAGE NAME TAG DIGEST SIZE dummyImageName tag DigestsA 123kB")
		So(err, ShouldBeNil)
		Convey("using shorthand", func() {
			args := []string{"imagetest", "-n", "dummyImageName", "--url", "someUrlImage"}
			buff := bytes.NewBufferString("")
			configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","showspinner":false}]}`)
			defer os.Remove(configPath)
			imageCmd := NewImageCommand(new(mockService))
			imageCmd.SetOut(buff)
			imageCmd.SetErr(buff)
			imageCmd.SetArgs(args)
			err := imageCmd.Execute()

			space := regexp.MustCompile(`\s+`)
			str := space.ReplaceAllString(buff.String(), " ")
			So(strings.TrimSpace(str), ShouldEqual, "IMAGE NAME TAG DIGEST SIZE dummyImageName tag DigestsA 123kB")
			So(err, ShouldBeNil)
		})
	})
}

func TestListRepos(t *testing.T) {
	Convey("Test listing repositories", t, func() {
		args := []string{"config-test"}

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewRepoCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldBeNil)
	})

	Convey("Test error on home directory", t, func() {
		args := []string{"config-test"}

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		err := os.Setenv("HOME", "nonExistentDirectory")
		if err != nil {
			panic(err)
		}

		cmd := NewRepoCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err = cmd.Execute()
		So(err, ShouldNotBeNil)

		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		err = os.Setenv("HOME", home)
		if err != nil {
			log.Fatal(err)
		}
	})

	Convey("Test listing repositories error", t, func() {
		args := []string{"config-test"}

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test",
		"url":"https://invalid.invalid","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewRepoCommand(new(searchService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
	})

	Convey("Test unable to get config value", t, func() {
		args := []string{"config-test-inexistent"}

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewRepoCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
	})

	Convey("Test error - no url provided", t, func() {
		args := []string{"config-test"}

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test","url":"","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewRepoCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
	})

	Convey("Test error - no args provided", t, func() {
		var args []string

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test","url":"","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewRepoCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
	})

	Convey("Test error - spinner config invalid", t, func() {
		args := []string{"config-test"}

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test",
		"url":"https://test-url.com","showspinner":invalid}]}`)
		defer os.Remove(configPath)

		cmd := NewRepoCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
	})

	Convey("Test error - verifyTLSConfig fails", t, func() {
		args := []string{"config-test"}

		configPath := makeConfigFile(`{"configs":[{"_name":"config-test", 
		"verify-tls":"invalid", "url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewRepoCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
	})
}

func TestOutputFormat(t *testing.T) {
	Convey("Test text", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "-o", "text"}

		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		space := regexp.MustCompile(`\s+`)
		str := space.ReplaceAllString(buff.String(), " ")
		So(strings.TrimSpace(str), ShouldEqual, "IMAGE NAME TAG DIGEST SIZE dummyImageName tag DigestsA 123kB")
		So(err, ShouldBeNil)
	})

	Convey("Test json", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "-o", "json"}

		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		space := regexp.MustCompile(`\s+`)
		str := space.ReplaceAllString(buff.String(), " ")
		So(strings.TrimSpace(str), ShouldEqual, `{ "name": "dummyImageName", "tags": [ { "name":`+
			` "tag", "size": 123445, "digest": "DigestsAreReallyLong", "configDigest": "", "layerDigests": null } ] }`)
		So(err, ShouldBeNil)
	})

	Convey("Test yaml", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "-o", "yaml"}

		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		space := regexp.MustCompile(`\s+`)
		str := space.ReplaceAllString(buff.String(), " ")
		So(strings.TrimSpace(str), ShouldEqual, `name: dummyImageName tags: -`+
			` name: tag size: 123445 digest: DigestsAreReallyLong configdigest: "" layers: []`)
		So(err, ShouldBeNil)

		Convey("Test yml", func() {
			args := []string{"imagetest", "--name", "dummyImageName", "-o", "yml"}

			configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","url":"https://test-url.com","showspinner":false}]}`)
			defer os.Remove(configPath)

			cmd := NewImageCommand(new(mockService))
			buff := bytes.NewBufferString("")
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err := cmd.Execute()
			space := regexp.MustCompile(`\s+`)
			str := space.ReplaceAllString(buff.String(), " ")
			So(strings.TrimSpace(str), ShouldEqual, `name: dummyImageName tags: -`+
				` name: tag size: 123445 digest: DigestsAreReallyLong configdigest: "" layers: []`)
			So(err, ShouldBeNil)
		})
	})

	Convey("Test invalid", t, func() {
		args := []string{"imagetest", "--name", "dummyImageName", "-o", "random"}

		configPath := makeConfigFile(`{"configs":[{"_name":"imagetest","url":"https://test-url.com","showspinner":false}]}`)
		defer os.Remove(configPath)

		cmd := NewImageCommand(new(mockService))
		buff := bytes.NewBufferString("")
		cmd.SetOut(buff)
		cmd.SetErr(buff)
		cmd.SetArgs(args)
		err := cmd.Execute()
		So(err, ShouldNotBeNil)
		So(buff.String(), ShouldContainSubstring, "invalid output format")
	})
}

func TestServerResponse(t *testing.T) {
	Convey("Test from real server", t, func() {
		port := test.GetFreePort()
		url := test.GetBaseURL(port)
		conf := config.New()
		conf.HTTP.Port = port
		defaultVal := true
		conf.Extensions = &extconf.ExtensionConfig{
			Search: &extconf.SearchConfig{Enable: &defaultVal},
		}
		ctlr := api.NewController(conf)
		ctlr.Config.Storage.RootDirectory = t.TempDir()
		go func(controller *api.Controller) {
			// this blocks
			if err := controller.Run(context.Background()); err != nil {
				return
			}
		}(ctlr)
		// wait till ready
		for {
			_, err := resty.R().Get(url)
			if err == nil {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
		defer func(controller *api.Controller) {
			ctx := context.Background()
			_ = controller.Server.Shutdown(ctx)
		}(ctlr)

		err := uploadManifest(url)
		t.Logf("%s", ctlr.Config.Storage.RootDirectory)
		So(err, ShouldBeNil)

		Convey("Test all images config url", func() {
			t.Logf("%s", ctlr.Config.Storage.RootDirectory)
			args := []string{"imagetest"}
			configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"imagetest","url":"%s","showspinner":false}]}`, url))
			defer os.Remove(configPath)
			cmd := NewImageCommand(new(searchService))
			// buff := bytes.NewBufferString("")
			buff := &bytes.Buffer{}
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err := cmd.Execute()
			So(err, ShouldBeNil)
			space := regexp.MustCompile(`\s+`)
			str := space.ReplaceAllString(buff.String(), " ")
			actual := strings.TrimSpace(str)
			So(actual, ShouldContainSubstring, "IMAGE NAME TAG DIGEST SIZE")
			So(actual, ShouldContainSubstring, "repo7 test:2.0 883fc0c5 15B")
			So(actual, ShouldContainSubstring, "repo7 test:1.0 883fc0c5 15B")
		})

		Convey("Test all images verbose", func() {
			args := []string{"imagetest", "--verbose"}
			configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"imagetest","url":"%s","showspinner":false}]}`, url))
			defer os.Remove(configPath)
			cmd := NewImageCommand(new(searchService))
			buff := bytes.NewBufferString("")
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err := cmd.Execute()
			So(err, ShouldBeNil)
			space := regexp.MustCompile(`\s+`)
			str := space.ReplaceAllString(buff.String(), " ")
			actual := strings.TrimSpace(str)
			// Actual cli output should be something similar to (order of images may differ):
			// IMAGE NAME    TAG       DIGEST    CONFIG    LAYERS    SIZE
			// repo7         test:2.0  a0ca253b  b8781e88            15B
			//                                             b8781e88  15B
			// repo7         test:1.0  a0ca253b  b8781e88            15B
			//                                             b8781e88  15B
			So(actual, ShouldContainSubstring, "IMAGE NAME TAG DIGEST CONFIG LAYERS SIZE")
			So(actual, ShouldContainSubstring, "repo7 test:2.0 883fc0c5 3a1d2d0c 15B b8781e88 15B")
			So(actual, ShouldContainSubstring, "repo7 test:1.0 883fc0c5 3a1d2d0c 15B b8781e88 15B")
		})

		Convey("Test image by name config url", func() {
			args := []string{"imagetest", "--name", "repo7"}
			configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"imagetest","url":"%s","showspinner":false}]}`, url))
			defer os.Remove(configPath)
			cmd := NewImageCommand(new(searchService))
			buff := bytes.NewBufferString("")
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err := cmd.Execute()
			So(err, ShouldBeNil)
			space := regexp.MustCompile(`\s+`)
			str := space.ReplaceAllString(buff.String(), " ")
			actual := strings.TrimSpace(str)
			So(actual, ShouldContainSubstring, "IMAGE NAME TAG DIGEST SIZE")
			So(actual, ShouldContainSubstring, "repo7 test:2.0 883fc0c5 15B")
			So(actual, ShouldContainSubstring, "repo7 test:1.0 883fc0c5 15B")

			Convey("with shorthand", func() {
				args := []string{"imagetest", "-n", "repo7"}
				configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"imagetest","url":"%s","showspinner":false}]}`, url))
				defer os.Remove(configPath)
				cmd := NewImageCommand(new(searchService))
				buff := bytes.NewBufferString("")
				cmd.SetOut(buff)
				cmd.SetErr(buff)
				cmd.SetArgs(args)
				err = cmd.Execute()
				So(err, ShouldBeNil)
				space := regexp.MustCompile(`\s+`)
				str := space.ReplaceAllString(buff.String(), " ")
				actual := strings.TrimSpace(str)
				So(actual, ShouldContainSubstring, "IMAGE NAME TAG DIGEST SIZE")
				So(actual, ShouldContainSubstring, "repo7 test:2.0 883fc0c5 15B")
				So(actual, ShouldContainSubstring, "repo7 test:1.0 883fc0c5 15B")
			})
		})

		Convey("Test image by digest", func() {
			args := []string{"imagetest", "--digest", "883fc0c5"}
			configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"imagetest","url":"%s","showspinner":false}]}`, url))
			defer os.Remove(configPath)
			cmd := NewImageCommand(new(searchService))
			buff := bytes.NewBufferString("")
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err := cmd.Execute()
			So(err, ShouldBeNil)
			space := regexp.MustCompile(`\s+`)
			str := space.ReplaceAllString(buff.String(), " ")
			actual := strings.TrimSpace(str)
			// Actual cli output should be something similar to (order of images may differ):
			// IMAGE NAME    TAG       DIGEST    SIZE
			// repo7         test:2.0  a0ca253b  15B
			// repo7         test:1.0  a0ca253b  15B
			So(actual, ShouldContainSubstring, "IMAGE NAME TAG DIGEST SIZE")
			So(actual, ShouldContainSubstring, "repo7 test:2.0 883fc0c5 15B")
			So(actual, ShouldContainSubstring, "repo7 test:1.0 883fc0c5 15B")
			Convey("with shorthand", func() {
				args := []string{"imagetest", "-d", "883fc0c5"}
				configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"imagetest","url":"%s","showspinner":false}]}`, url))
				defer os.Remove(configPath)
				cmd := NewImageCommand(new(searchService))
				buff := bytes.NewBufferString("")
				cmd.SetOut(buff)
				cmd.SetErr(buff)
				cmd.SetArgs(args)
				err = cmd.Execute()
				So(err, ShouldBeNil)
				space := regexp.MustCompile(`\s+`)
				str := space.ReplaceAllString(buff.String(), " ")
				actual := strings.TrimSpace(str)
				So(actual, ShouldContainSubstring, "IMAGE NAME TAG DIGEST SIZE")
				So(actual, ShouldContainSubstring, "repo7 test:2.0 883fc0c5 15B")
				So(actual, ShouldContainSubstring, "repo7 test:1.0 883fc0c5 15B")
			})
		})

		Convey("Test image by name invalid name", func() {
			args := []string{"imagetest", "--name", "repo777"}
			configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"imagetest","url":"%s","showspinner":false}]}`, url))
			defer os.Remove(configPath)
			cmd := NewImageCommand(new(searchService))
			buff := bytes.NewBufferString("")
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err := cmd.Execute()
			So(err, ShouldNotBeNil)
			actual := buff.String()
			So(actual, ShouldContainSubstring, "unknown")
		})

		Convey("Test list repos error", func() {
			args := []string{"config-test"}

			configPath := makeConfigFile(fmt.Sprintf(`{"configs":[{"_name":"config-test",
			"url":"%s","showspinner":false}]}`, url))
			defer os.Remove(configPath)

			cmd := NewRepoCommand(new(searchService))
			buff := &bytes.Buffer{}
			cmd.SetOut(buff)
			cmd.SetErr(buff)
			cmd.SetArgs(args)
			err = cmd.Execute()
			space := regexp.MustCompile(`\s+`)
			str := space.ReplaceAllString(buff.String(), " ")
			actual := strings.TrimSpace(str)

			So(actual, ShouldContainSubstring, "REPOSITORY NAME")
			So(actual, ShouldContainSubstring, "repo7")
		})
	})
}

func uploadManifest(url string) error {
	// create a blob/layer
	resp, _ := resty.R().Post(url + "/v2/repo7/blobs/uploads/")
	loc := test.Location(url, resp)

	content := []byte("this is a blob5")
	digest := godigest.FromBytes(content)
	_, _ = resty.R().SetQueryParam("digest", digest.String()).
		SetHeader("Content-Type", "application/octet-stream").SetBody(content).Put(loc)

	// upload image config blob
	resp, _ = resty.R().Post(url + "/v2/repo7/blobs/uploads/")
	loc = test.Location(url, resp)
	cblob, cdigest := test.GetImageConfig()

	_, _ = resty.R().
		SetContentLength(true).
		SetHeader("Content-Length", fmt.Sprintf("%d", len(cblob))).
		SetHeader("Content-Type", "application/octet-stream").
		SetQueryParam("digest", cdigest.String()).
		SetBody(cblob).
		Put(loc)

	// create a manifest
	manifest := ispec.Manifest{
		Config: ispec.Descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    cdigest,
			Size:      int64(len(cblob)),
		},
		Layers: []ispec.Descriptor{
			{
				MediaType: "application/vnd.oci.image.layer.v1.tar",
				Digest:    digest,
				Size:      int64(len(content)),
			},
		},
	}
	manifest.SchemaVersion = 2

	content, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	_, _ = resty.R().SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
		SetBody(content).Put(url + "/v2/repo7/manifests/test:1.0")

	content = []byte("this is a blob5")
	digest = godigest.FromBytes(content)
	// create a manifest with same blob but a different tag
	manifest = ispec.Manifest{
		Config: ispec.Descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    cdigest,
			Size:      int64(len(cblob)),
		},
		Layers: []ispec.Descriptor{
			{
				MediaType: "application/vnd.oci.image.layer.v1.tar",
				Digest:    digest,
				Size:      int64(len(content)),
			},
		},
	}
	manifest.SchemaVersion = 2

	content, err = json.Marshal(manifest)
	if err != nil {
		return err
	}
	_, _ = resty.R().SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
		SetBody(content).Put(url + "/v2/repo7/manifests/test:2.0")

	return nil
}

type mockService struct{}

func (service mockService) getRepos(ctx context.Context, config searchConfig, username,
	password string, channel chan stringResult, wtgrp *sync.WaitGroup,
) {
	defer wtgrp.Done()
	defer close(channel)

	var catalog [3]string
	catalog[0] = "python"
	catalog[1] = "busybox"
	catalog[2] = "hello-world"

	channel <- stringResult{"", nil}
}

func (service mockService) getAllImages(ctx context.Context, config searchConfig, username, password string,
	channel chan stringResult, wtgrp *sync.WaitGroup,
) {
	defer wtgrp.Done()
	defer close(channel)

	image := &imageStruct{}
	image.Name = "randomimageName"
	image.Tags = []tags{
		{
			Name:   "tag",
			Digest: "DigestsAreReallyLong",
			Size:   123445,
		},
	}

	str, err := image.string(*config.outputFormat)
	if err != nil {
		channel <- stringResult{"", err}

		return
	}

	channel <- stringResult{str, nil}
}

func (service mockService) getImageByName(ctx context.Context, config searchConfig,
	username, password, imageName string, channel chan stringResult, wtgrp *sync.WaitGroup,
) {
	defer wtgrp.Done()
	defer close(channel)

	image := &imageStruct{}
	image.Name = imageName
	image.Tags = []tags{
		{
			Name:   "tag",
			Digest: "DigestsAreReallyLong",
			Size:   123445,
		},
	}

	str, err := image.string(*config.outputFormat)
	if err != nil {
		channel <- stringResult{"", err}

		return
	}

	channel <- stringResult{str, nil}
}

func (service mockService) getCveByImage(ctx context.Context, config searchConfig, username, password,
	imageName string, rch chan stringResult, wtgrp *sync.WaitGroup,
) {
	defer wtgrp.Done()
	defer close(rch)

	cveRes := &cveResult{}
	cveRes.Data = cveData{
		CVEListForImage: cveListForImage{
			Tag: imageName,
			CVEList: []cve{
				{
					ID:          "dummyCVEID",
					Description: "Description of the CVE",
					Title:       "Title of that CVE",
					Severity:    "HIGH",
					PackageList: []packageList{
						{
							Name:             "packagename",
							FixedVersion:     "fixedver",
							InstalledVersion: "installedver",
						},
					},
				},
			},
		},
	}

	str, err := cveRes.string(*config.outputFormat)
	if err != nil {
		rch <- stringResult{"", err}

		return
	}

	rch <- stringResult{str, nil}
}

func (service mockService) getImagesByCveID(ctx context.Context, config searchConfig, username, password, cvid string,
	rch chan stringResult, wtgrp *sync.WaitGroup,
) {
	service.getImageByName(ctx, config, username, password, "anImage", rch, wtgrp)
}

func (service mockService) getImagesByDigest(ctx context.Context, config searchConfig, username,
	password, digest string, rch chan stringResult, wtgrp *sync.WaitGroup,
) {
	service.getImageByName(ctx, config, username, password, "anImage", rch, wtgrp)
}

func (service mockService) getImageByNameAndCVEID(ctx context.Context, config searchConfig, username,
	password, imageName, cvid string, rch chan stringResult, wtgrp *sync.WaitGroup,
) {
	service.getImageByName(ctx, config, username, password, imageName, rch, wtgrp)
}

func (service mockService) getFixedTagsForCVE(ctx context.Context, config searchConfig,
	username, password, imageName, cvid string, rch chan stringResult, wtgrp *sync.WaitGroup,
) {
	service.getImageByName(ctx, config, username, password, imageName, rch, wtgrp)
}

func makeConfigFile(content string) string {
	os.Setenv("HOME", os.TempDir())

	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	configPath := path.Join(home + "/.zot")

	if err := ioutil.WriteFile(configPath, []byte(content), 0o600); err != nil {
		panic(err)
	}

	return configPath
}
