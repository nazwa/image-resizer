// Copyright 2013 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Taken from https://github.com/willnorris/imageproxy
//

package main

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"willnorris.com/go/imageproxy"
)

const (
	optFit             = "fit"
	optFlipVertical    = "fv"
	optFlipHorizontal  = "fh"
	optRotatePrefix    = "r"
	optQualityPrefix   = "q"
	optSignaturePrefix = "s"
	optSizeDelimiter   = "x"
	optScaleUp         = "scaleUp"
)

// ParseOptions parses str as a list of comma separated transformation options.
// The following options can be specified in any order:
//
// Size and Cropping
//
// The size option takes the general form "{width}x{height}", where width and
// height are numbers. Integer values greater than 1 are interpreted as exact
// pixel values. Floats between 0 and 1 are interpreted as percentages of the
// original image size. If either value is omitted or set to 0, it will be
// automatically set to preserve the aspect ratio based on the other dimension.
// If a single number is provided (with no "x" separator), it will be used for
// both height and width.
//
// Depending on the size options specified, an image may be cropped to fit the
// requested size. In all cases, the original aspect ratio of the image will be
// preserved; imageproxy will never stretch the original image.
//
// When no explicit crop mode is specified, the following rules are followed:
//
// - If both width and height values are specified, the image will be scaled to
// fill the space, cropping if necessary to fit the exact dimension.
//
// - If only one of the width or height values is specified, the image will be
// resized to fit the specified dimension, scaling the other dimension as
// needed to maintain the aspect ratio.
//
// If the "fit" option is specified together with a width and height value, the
// image will be resized to fit within a containing box of the specified size.
// As always, the original aspect ratio will be preserved. Specifying the "fit"
// option with only one of either width or height does the same thing as if
// "fit" had not been specified.
//
// Rotation and Flips
//
// The "r{degrees}" option will rotate the image the specified number of
// degrees, counter-clockwise. Valid degrees values are 90, 180, and 270.
//
// The "fv" option will flip the image vertically. The "fh" option will flip
// the image horizontally. Images are flipped after being rotated.
//
// Quality
//
// The "q{qualityPercentage}" option can be used to specify the quality of the
// output file (JPEG only)
//
// Examples
//
// 	0x0       - no resizing
// 	200x      - 200 pixels wide, proportional height
// 	0.15x     - 15% original width, proportional height
// 	x100      - 100 pixels tall, proportional width
// 	100x150   - 100 by 150 pixels, cropping as needed
// 	100       - 100 pixels square, cropping as needed
// 	150,fit   - scale to fit 150 pixels square, no cropping
// 	100,r90   - 100 pixels square, rotated 90 degrees
// 	100,fv,fh - 100 pixels square, flipped horizontal and vertical
// 	200x,q80  - 200 pixels wide, proportional height, 80% quality
func ParseOptions(str string) imageproxy.Options {
	var options imageproxy.Options

	for _, opt := range strings.Split(str, ",") {
		switch {
		case len(opt) == 0:
			break
		case opt == optFit:
			options.Fit = true
		case opt == optFlipVertical:
			options.FlipVertical = true
		case opt == optFlipHorizontal:
			options.FlipHorizontal = true
		case opt == optScaleUp: // this option is intentionally not documented above
			options.ScaleUp = true
		case strings.HasPrefix(opt, optRotatePrefix):
			value := strings.TrimPrefix(opt, optRotatePrefix)
			options.Rotate, _ = strconv.Atoi(value)
		case strings.HasPrefix(opt, optQualityPrefix):
			value := strings.TrimPrefix(opt, optQualityPrefix)
			options.Quality, _ = strconv.Atoi(value)
		case strings.HasPrefix(opt, optSignaturePrefix):
			options.Signature = strings.TrimPrefix(opt, optSignaturePrefix)
		case strings.Contains(opt, optSizeDelimiter):
			size := strings.SplitN(opt, optSizeDelimiter, 2)
			if w := size[0]; w != "" {
				options.Width, _ = strconv.ParseFloat(w, 64)
			}
			if h := size[1]; h != "" {
				options.Height, _ = strconv.ParseFloat(h, 64)
			}
		default:
			if size, err := strconv.ParseFloat(opt, 64); err == nil {
				options.Width = size
				options.Height = size
			}
		}
	}

	return options
}

//	path := r.URL.Path[1:] // strip leading slash
//	req.URL, err = parseURL(path)
//	if err != nil || !req.URL.IsAbs() {
//		// first segment should be options
//		parts := strings.SplitN(path, "/", 2)
//		if len(parts) != 2 {
//			return nil, URLError{"too few path segments", r.URL}
//		}

//		var err error
//		req.URL, err = parseURL(parts[1])
//		if err != nil {
//			return nil, URLError{fmt.Sprintf("unable to parse remote URL: %v", err), r.URL}
//		}

//		req.Options = ParseOptions(parts[0])
//	}

var reCleanedURL = regexp.MustCompile(`^(https?):/+([^/])`)

// parseURL parses s as a URL, handling URLs that have been munged by
// path.Clean or a webserver that collapses multiple slashes.
func parseURL(s string) (*url.URL, error) {
	s = reCleanedURL.ReplaceAllString(s, "$1://$2")
	return url.Parse(s)
}
