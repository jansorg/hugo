// Copyright 2018 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package svg

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/gohugoio/hugo/common/herrors"
	"github.com/gohugoio/hugo/hugolib/filesystems"
	"github.com/gohugoio/hugo/media"
	"github.com/gohugoio/hugo/resources"
	"github.com/gohugoio/hugo/resources/internal"
	"github.com/gohugoio/hugo/resources/resource"
	"github.com/mitchellh/mapstructure"
)

// Some of the options from https://inkscape.org/en/doc/inkscape-man.html#OPTIONS
type Options struct {
	TargetPath string
	Width      string
	Height     string
}

func DecodeOptions(m map[string]interface{}) (opts Options, err error) {
	if m == nil {
		return
	}
	err = mapstructure.WeakDecode(m, &opts)
	return
}

func (opts Options) toArgs() []string {
	var args []string
	if opts.Width != "" {
		args = append(args, "-w", opts.Width)
	}
	if opts.Height != "" {
		args = append(args, "-h", opts.Height)
	}
	return args
}

// Client is the client used to do SVG transformations.
type Client struct {
	sfs *filesystems.SourceFilesystem
	rs  *resources.Spec
}

// New creates a new Client with the given specification.
func New(fs *filesystems.SourceFilesystem, rs *resources.Spec) *Client {
	return &Client{sfs: fs, rs: rs}
}

type svgTransformation struct {
	c       *Client
	rs      *resources.Spec
	options Options
}

func (t *svgTransformation) Key() internal.ResourceTransformationKey {
	return internal.NewResourceTransformationKey("svgToPng", t.options)
}

// Transform shells out to inkscape to do the heavy lifting.
// For this to work, you need to have the inkscape binary installed.
func (t *svgTransformation) Transform(ctx *resources.ResourceTransformationCtx) error {
	const binaryName = "inkscape"

	if _, err := exec.LookPath(binaryName); err != nil {
		// This may be on a CI server etc. Will fall back to pre-built assets.
		return herrors.ErrFeatureNotAvailable
	}

	ctx.OutMediaType = media.PNGType
	if t.options.TargetPath != "" {
		ctx.OutPath = t.options.TargetPath
	} else {
		var ext string
		if t.options.Width != "" && t.options.Height != "" {
			ext = fmt.Sprintf("-%sx%s.png", t.options.Width, t.options.Height)
		} else if t.options.Width != "" {
			ext = fmt.Sprintf("-%s.png", t.options.Width)
		} else if t.options.Height != "" {
			ext = fmt.Sprintf("-%s.png", t.options.Height)
		} else {
			ext = ".png"
		}

		ctx.ReplaceOutPathExtension(ext)
	}

	var cmdArgs []string
	if optArgs := t.options.toArgs(); len(optArgs) > 0 {
		cmdArgs = append(cmdArgs, optArgs...)
	}

	// create temp files for input and output
	tempInput, err := ioutil.TempFile("", "hugo-svg-in*.svg")
	if err != nil {
		return err
	}
	defer os.Remove(tempInput.Name())

	tempOutput, err := ioutil.TempFile("", "hugo-svg-out*.png")
	if err != nil {
		return err
	}
	_ = tempOutput.Close()
	defer os.Remove(tempOutput.Name())

	if _, err = io.Copy(tempInput, ctx.From); err != nil {
		return err
	}
	_ = tempInput.Close()

	cmdArgs = append(cmdArgs, "-e", tempOutput.Name())
	cmdArgs = append(cmdArgs, tempInput.Name())

	//fmt.Printf("svg exec %v, source %s\n", cmdArgs, ctx.SourcePath)
	cmd := exec.Command(binaryName, cmdArgs...)
	//cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		//fmt.Printf("svg failed, %s\n", string(err.(*exec.ExitError).Stderr))
		return err
	}

	tempOutputName, err := os.Open(tempOutput.Name())
	if err != nil {
		return err
	}
	defer tempOutputName.Close()
	_, _ = io.Copy(ctx.To, tempOutputName)

	return nil
}

// Process transforms the given Resource with the PostCSS processor.
func (c *Client) Process(res resources.ResourceTransformer, options Options) (resource.Resource, error) {
	return res.Transform(&svgTransformation{c: c, rs: c.rs, options: options})
}
