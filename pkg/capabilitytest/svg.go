package capabilitytest

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Shape struct {
	Tag  string
	Attr map[string]string
	Text string
}

var paintPattern = regexp.MustCompile(`^(#[0-9a-fA-F]{3}|#[0-9a-fA-F]{6}|none|black|white|red|blue|green|orange|yellow|purple|brown|gray|pink|navy|teal)$`)
var numericPattern = regexp.MustCompile(`^[0-9eE+.,\s-]+$`)
var pathPattern = regexp.MustCompile(`^[MmZzLlHhVvCcSsQqTtAa0-9eE+.,\s-]+$`)
var transformPattern = regexp.MustCompile(`^(\s*(translate|rotate|scale)\([0-9eE+.,\s-]+\)\s*)+$`)

// ValidateSVG rejects rather than repairs dangerous or unsupported content.
// Sanitized output is re-encoded; the original is never embedded in HTML.
func ValidateSVG(answer string, geometry bool) ([]byte, []Shape, error) {
	answer = Unfence(answer)
	if len(answer) > MaxSVGBytes {
		return nil, nil, errors.New("svg_too_large")
	}
	d := xml.NewDecoder(strings.NewReader(answer))
	var output bytes.Buffer
	e := xml.NewEncoder(&output)
	depth, count, roots := 0, 0, 0
	stack := []string{}
	shapes := []Shape{}
	textIndex := -1
	allowed := map[string]bool{"svg": true, "g": true, "path": true, "rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true, "polygon": true, "text": true, "tspan": true}
	numeric := map[string]bool{"x": true, "y": true, "x1": true, "y1": true, "x2": true, "y2": true, "width": true, "height": true, "cx": true, "cy": true, "r": true, "rx": true, "ry": true, "font-size": true, "stroke-width": true, "opacity": true, "fill-opacity": true, "stroke-opacity": true, "dx": true, "dy": true}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, errors.New("invalid_svg")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			count++
			depth++
			if count > 2000 || depth > 32 || !allowed[t.Name.Local] || (t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg") {
				return nil, nil, errors.New("unsupported_svg")
			}
			if depth == 1 {
				roots++
				if roots != 1 || t.Name.Local != "svg" {
					return nil, nil, errors.New("invalid_svg_root")
				}
			} else if t.Name.Local == "svg" {
				return nil, nil, errors.New("nested_svg")
			}
			if geometry && depth > 1 && (depth != 2 || (t.Name.Local != "circle" && t.Name.Local != "rect" && t.Name.Local != "text")) {
				return nil, nil, errors.New("unsupported_geometry")
			}
			a := map[string]string{}
			attrs := []xml.Attr{}
			for _, attr := range t.Attr {
				k, v := attr.Name.Local, attr.Value
				if k == "xmlns" && v == "http://www.w3.org/2000/svg" {
					continue
				}
				if attr.Name.Space != "" {
					return nil, nil, errors.New("unsupported_svg_attribute")
				}
				if _, exists := a[k]; exists {
					return nil, nil, errors.New("duplicate_attribute")
				}
				a[k] = v
				valid := false
				switch {
				case k == "fill" || k == "stroke":
					valid = paintPattern.MatchString(v)
				case numeric[k]:
					n, err := strconv.ParseFloat(v, 64)
					valid = err == nil && !math.IsInf(n, 0) && !math.IsNaN(n) && math.Abs(n) <= 10000
				case k == "viewBox":
					valid = depth == 1 && v == "0 0 600 400"
				case k == "d":
					valid = !geometry && pathPattern.MatchString(v)
				case k == "points":
					valid = !geometry && numericPattern.MatchString(v)
				case k == "transform":
					valid = !geometry && transformPattern.MatchString(v)
				case k == "text-anchor":
					valid = v == "start" || v == "middle" || v == "end"
				case k == "stroke-linecap":
					valid = v == "round" || v == "butt" || v == "square"
				case k == "stroke-linejoin":
					valid = v == "round" || v == "miter" || v == "bevel"
				}
				if geometry && (k == "stroke" || k == "stroke-width" || strings.Contains(k, "opacity")) {
					valid = false
				}
				if !valid {
					return nil, nil, errors.New("unsupported_svg_attribute")
				}
				attrs = append(attrs, xml.Attr{Name: xml.Name{Local: k}, Value: v})
			}
			if depth == 1 && (a["width"] != "600" || a["height"] != "400" || a["viewBox"] != "0 0 600 400") {
				return nil, nil, errors.New("invalid_canvas")
			}
			clean := xml.StartElement{Name: xml.Name{Local: t.Name.Local}, Attr: attrs}
			if depth == 1 {
				clean.Attr = append(clean.Attr, xml.Attr{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/svg"})
			}
			if err = e.EncodeToken(clean); err != nil {
				return nil, nil, err
			}
			stack = append(stack, t.Name.Local)
			shapes = append(shapes, Shape{Tag: t.Name.Local, Attr: a})
			if t.Name.Local == "text" {
				textIndex = len(shapes) - 1
			}
		case xml.EndElement:
			if t.Name.Local == "text" {
				textIndex = -1
			}
			depth--
			stack = stack[:len(stack)-1]
			if err = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: t.Name.Local}}); err != nil {
				return nil, nil, err
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				if len(stack) == 0 || (stack[len(stack)-1] != "text" && stack[len(stack)-1] != "tspan") {
					return nil, nil, errors.New("unexpected_svg_text")
				}
				if textIndex >= 0 {
					shapes[textIndex].Text += string(t)
				}
			}
			if err = e.EncodeToken(t); err != nil {
				return nil, nil, err
			}
		case xml.Comment: // Comments are neither instructions nor rendered content.
		case xml.ProcInst:
			if depth != 0 || roots != 0 || t.Target != "xml" {
				return nil, nil, errors.New("unsupported_svg_instruction")
			}
		default:
			return nil, nil, errors.New("unsupported_svg_instruction")
		}
	}
	if roots != 1 || depth != 0 {
		return nil, nil, errors.New("invalid_svg")
	}
	if err := e.Flush(); err != nil {
		return nil, nil, err
	}
	return output.Bytes(), shapes, nil
}
