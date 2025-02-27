//go:build search
// +build search

// nolint: gochecknoinits
package digestinfo_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/resty.v1"
	"zotregistry.io/zot/pkg/api"
	"zotregistry.io/zot/pkg/api/config"
	"zotregistry.io/zot/pkg/api/constants"
	extconf "zotregistry.io/zot/pkg/extensions/config"
	"zotregistry.io/zot/pkg/extensions/monitoring"
	digestinfo "zotregistry.io/zot/pkg/extensions/search/digest"
	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/storage"
	. "zotregistry.io/zot/pkg/test"
)

// nolint:gochecknoglobals
var (
	digestInfo *digestinfo.DigestInfo
	rootDir    string
	subRootDir string
)

type ImgResponseForDigest struct {
	ImgListForDigest ImgListForDigest `json:"data"`
	Errors           []ErrorGQL       `json:"errors"`
}

//nolint:tagliatelle // graphQL schema
type ImgListForDigest struct {
	Images []ImgInfo `json:"ImageListForDigest"`
}

//nolint:tagliatelle // graphQL schema
type ImgInfo struct {
	Name string   `json:"Name"`
	Tags []string `json:"Tags"`
}

type ErrorGQL struct {
	Message string   `json:"message"`
	Path    []string `json:"path"`
}

func init() {
	if err := testSetup(); err != nil {
		panic(err)
	}
}

func testSetup() error {
	dir, err := ioutil.TempDir("", "digest_test")
	if err != nil {
		return err
	}

	subDir, err := ioutil.TempDir("", "sub_digest_test")
	if err != nil {
		return err
	}

	rootDir = dir

	subRootDir = subDir

	// Test images used/copied:
	// IMAGE NAME    TAG                       DIGEST    CONFIG    LAYERS    SIZE
	// zot-test      0.0.1                     2bacca16  adf3bb6c            76MB
	//                                                             2d473b07  76MB
	// zot-cve-test  0.0.1                     63a795ca  8dd57e17            75MB
	//                                                             7a0437f0  75MB

	err = os.Mkdir(subDir+"/a", 0o700)
	if err != nil {
		return err
	}

	err = CopyFiles("../../../../test/data", rootDir)
	if err != nil {
		return err
	}

	err = CopyFiles("../../../../test/data", subDir+"/a/")
	if err != nil {
		return err
	}

	conf := config.New()
	conf.Extensions = &extconf.ExtensionConfig{}
	conf.Extensions.Lint = &extconf.LintConfig{}

	log := log.NewLogger("debug", "")
	metrics := monitoring.NewMetricsServer(false, log)
	storeController := storage.StoreController{
		DefaultStore: storage.NewImageStore(rootDir, false, storage.DefaultGCDelay,
			false, false, log, metrics, nil),
	}

	digestInfo = digestinfo.NewDigestInfo(storeController, log)

	return nil
}

func TestDigestInfo(t *testing.T) {
	Convey("Test image tag", t, func() {
		// Search by manifest digest
		var (
			manifestDigest digest.Digest
			configDigest   digest.Digest
			layerDigest    digest.Digest
		)

		manifestDigest, _, layerDigest = GetOciLayoutDigests("../../../../test/data/zot-cve-test")

		imageTags, err := digestInfo.GetImageTagsByDigest("zot-cve-test", string(manifestDigest))
		So(err, ShouldBeNil)
		So(len(imageTags), ShouldEqual, 1)
		So(*imageTags[0], ShouldEqual, "0.0.1")

		// Search by config digest
		_, configDigest, _ = GetOciLayoutDigests("../../../../test/data/zot-test")

		imageTags, err = digestInfo.GetImageTagsByDigest("zot-test", string(configDigest))
		So(err, ShouldBeNil)
		So(len(imageTags), ShouldEqual, 1)
		So(*imageTags[0], ShouldEqual, "0.0.1")

		// Search by layer digest
		imageTags, err = digestInfo.GetImageTagsByDigest("zot-cve-test", string(layerDigest))
		So(err, ShouldBeNil)
		So(len(imageTags), ShouldEqual, 1)
		So(*imageTags[0], ShouldEqual, "0.0.1")

		// Search by non-existent image
		imageTags, err = digestInfo.GetImageTagsByDigest("zot-tes", "63a795ca")
		So(err, ShouldNotBeNil)
		So(len(imageTags), ShouldEqual, 0)

		// Search by non-existent digest
		imageTags, err = digestInfo.GetImageTagsByDigest("zot-test", "111")
		So(err, ShouldBeNil)
		So(len(imageTags), ShouldEqual, 0)
	})
}

func TestDigestSearchHTTP(t *testing.T) {
	Convey("Test image search by digest scanning", t, func() {
		port := GetFreePort()
		baseURL := GetBaseURL(port)
		conf := config.New()
		conf.HTTP.Port = port
		conf.Storage.RootDirectory = rootDir
		defaultVal := true
		conf.Extensions = &extconf.ExtensionConfig{
			Search: &extconf.SearchConfig{Enable: &defaultVal},
		}

		ctlr := api.NewController(conf)

		go func() {
			// this blocks
			if err := ctlr.Run(context.Background()); err != nil {
				return
			}
		}()

		// wait till ready
		for {
			_, err := resty.R().Get(baseURL)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// shut down server
		defer func() {
			ctx := context.Background()
			_ = ctlr.Server.Shutdown(ctx)
		}()

		resp, err := resty.R().Get(baseURL + "/v2/")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix)
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 422)

		// "sha" should match all digests in all images
		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix +
			"?query={ImageListForDigest(id:\"sha\"){Name%20Tags}}")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		var responseStruct ImgResponseForDigest
		err = json.Unmarshal(resp.Body(), &responseStruct)
		So(err, ShouldBeNil)
		So(len(responseStruct.Errors), ShouldEqual, 0)
		So(len(responseStruct.ImgListForDigest.Images), ShouldEqual, 2)
		So(len(responseStruct.ImgListForDigest.Images[0].Tags), ShouldEqual, 1)
		So(len(responseStruct.ImgListForDigest.Images[0].Tags), ShouldEqual, 1)

		// Call should return {"data":{"ImageListForDigest":[{"Name":"zot-test","Tags":["0.0.1"]}]}}
		var layerDigest digest.Digest
		var manifestDigest digest.Digest
		manifestDigest, _, layerDigest = GetOciLayoutDigests("../../../../test/data/zot-test")

		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix + "?query={ImageListForDigest(id:\"" +
			string(layerDigest) + "\"){Name%20Tags}}")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		err = json.Unmarshal(resp.Body(), &responseStruct)
		So(err, ShouldBeNil)
		So(len(responseStruct.Errors), ShouldEqual, 0)
		So(len(responseStruct.ImgListForDigest.Images), ShouldEqual, 1)
		So(responseStruct.ImgListForDigest.Images[0].Name, ShouldEqual, "zot-test")
		So(len(responseStruct.ImgListForDigest.Images[0].Tags), ShouldEqual, 1)
		So(responseStruct.ImgListForDigest.Images[0].Tags[0], ShouldEqual, "0.0.1")

		// Call should return {"data":{"ImageListForDigest":[{"Name":"zot-test","Tags":["0.0.1"]}]}}

		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix +
			"?query={ImageListForDigest(id:\"" +
			string(manifestDigest) + "\"){Name%20Tags}}")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		err = json.Unmarshal(resp.Body(), &responseStruct)
		So(err, ShouldBeNil)
		So(len(responseStruct.Errors), ShouldEqual, 0)
		So(len(responseStruct.ImgListForDigest.Images), ShouldEqual, 1)
		So(responseStruct.ImgListForDigest.Images[0].Name, ShouldEqual, "zot-test")
		So(len(responseStruct.ImgListForDigest.Images[0].Tags), ShouldEqual, 1)
		So(responseStruct.ImgListForDigest.Images[0].Tags[0], ShouldEqual, "0.0.1")

		// Call should return {"data":{"ImageListForDigest":[{"Name":"zot-cve-test","Tags":["0.0.1"]}]}}

		_, _, layerDigest = GetOciLayoutDigests("../../../../test/data/zot-cve-test")
		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix + "?query={ImageListForDigest(id:\"" +
			string(layerDigest) + "\"){Name%20Tags}}")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		err = json.Unmarshal(resp.Body(), &responseStruct)
		So(err, ShouldBeNil)
		So(len(responseStruct.Errors), ShouldEqual, 0)
		So(len(responseStruct.ImgListForDigest.Images), ShouldEqual, 1)
		So(responseStruct.ImgListForDigest.Images[0].Name, ShouldEqual, "zot-cve-test")
		So(len(responseStruct.ImgListForDigest.Images[0].Tags), ShouldEqual, 1)
		So(responseStruct.ImgListForDigest.Images[0].Tags[0], ShouldEqual, "0.0.1")

		// Call should return {"data":{"ImageListForDigest":[]}}
		// "1111111" should match 0 images
		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix +
			"?query={ImageListForDigest(id:\"1111111\"){Name%20Tags}}")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		err = json.Unmarshal(resp.Body(), &responseStruct)
		So(err, ShouldBeNil)
		So(len(responseStruct.Errors), ShouldEqual, 0)
		So(len(responseStruct.ImgListForDigest.Images), ShouldEqual, 0)

		// Call should return {"errors": [{....}]", data":null}}
		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix +
			"?query={ImageListForDigest(id:\"1111111\"){Name%20Tag343s}}")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 422)

		err = json.Unmarshal(resp.Body(), &responseStruct)
		So(err, ShouldBeNil)
		So(len(responseStruct.Errors), ShouldEqual, 1)
	})
}

func TestDigestSearchHTTPSubPaths(t *testing.T) {
	Convey("Test image search by digest scanning using storage subpaths", t, func() {
		port := GetFreePort()
		baseURL := GetBaseURL(port)
		conf := config.New()
		conf.HTTP.Port = port
		defaultVal := true
		conf.Extensions = &extconf.ExtensionConfig{
			Search: &extconf.SearchConfig{Enable: &defaultVal},
		}

		ctlr := api.NewController(conf)

		globalDir, err := ioutil.TempDir("", "digest_test")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(globalDir)

		ctlr.Config.Storage.RootDirectory = globalDir

		subPathMap := make(map[string]config.StorageConfig)

		subPathMap["/a"] = config.StorageConfig{RootDirectory: subRootDir}

		ctlr.Config.Storage.SubPaths = subPathMap

		go func() {
			// this blocks
			if err := ctlr.Run(context.Background()); err != nil {
				return
			}
		}()

		// wait till ready
		for {
			_, err := resty.R().Get(baseURL)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// shut down server
		defer func() {
			ctx := context.Background()
			_ = ctlr.Server.Shutdown(ctx)
		}()

		resp, err := resty.R().Get(baseURL + "/v2/")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix)
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 422)

		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix +
			"?query={ImageListForDigest(id:\"sha\"){Name%20Tags}}")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		var responseStruct ImgResponseForDigest
		err = json.Unmarshal(resp.Body(), &responseStruct)
		So(err, ShouldBeNil)
		So(len(responseStruct.Errors), ShouldEqual, 0)
		So(len(responseStruct.ImgListForDigest.Images), ShouldEqual, 2)
	})
}

func TestDigestSearchDisabled(t *testing.T) {
	Convey("Test disabling image search", t, func() {
		var disabled bool
		port := GetFreePort()
		baseURL := GetBaseURL(port)
		conf := config.New()
		conf.HTTP.Port = port
		conf.Storage.RootDirectory = t.TempDir()
		conf.Extensions = &extconf.ExtensionConfig{
			Search: &extconf.SearchConfig{Enable: &disabled},
		}

		ctlr := api.NewController(conf)

		go func() {
			// this blocks
			if err := ctlr.Run(context.Background()); err != nil {
				return
			}
		}()

		// wait till ready
		for {
			_, err := resty.R().Get(baseURL)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// shut down server
		defer func() {
			ctx := context.Background()
			_ = ctlr.Server.Shutdown(ctx)
		}()

		resp, err := resty.R().Get(baseURL + "/v2/")
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 200)

		resp, err = resty.R().Get(baseURL + constants.ExtSearchPrefix)
		So(resp, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(resp.StatusCode(), ShouldEqual, 404)
	})
}
