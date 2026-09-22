package capabilitytest

import (
	"fmt"
	"strings"
	"testing"
)

func TestLogicStrictAnswer(t *testing.T) {
	for _, input := range []string{`29`, `{"answers":[{"id":"anchor","value":"29"},{"id":"random","value":29}]}`, `{"answers":[{"id":"anchor","value":29},{"id":"anchor","value":29}]}`, `{"answers":[{"id":"anchor","value":29.5},{"id":"random","value":29}]}`, `答案是21或29`} {
		if _, err := ParseLogic(input, 29); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	got, err := ParseLogic("```json\n{\"answers\":[{\"id\":\"random\",\"value\":35},{\"id\":\"anchor\",\"value\":29}]}\n```", 35)
	if err != nil || !got[0] || !got[1] {
		t.Fatal(got, err)
	}
}
func TestQuestionsDeterministicAndOracle(t *testing.T) {
	for i := 0; i < 1000; i++ {
		q := Generate(fmt.Sprint(i))
		if q[2].Format != "html-svg-animation" || len(q[2].Requirements) != 6 || q[2].TemplateID == "" {
			t.Fatal("invalid animation question")
		}
		again := Generate(fmt.Sprint(i))
		if q[0].Hash != again[0].Hash {
			t.Fatal("not frozen")
		}
		var a, b, c, d, w int
		_, err := fmt.Sscanf(strings.Split(q[0].Prompt, "第二题条件相同，但")[1], "圆形苹果糖%d颗、圆形桃子糖%d颗、星形苹果糖%d颗、星形桃子糖%d颗，西瓜糖%d颗", &a, &b, &c, &d, &w)
		if err != nil {
			t.Fatal(err)
		}
		largest := 0
		// Independent enumeration of legal non-pair selections.
		for ac := 0; ac <= a; ac++ {
			for pc := 0; pc <= b; pc++ {
				for as := 0; as <= c; as++ {
					for ps := 0; ps <= d; ps++ {
						if (ac == 0 || as == 0) && (pc == 0 || ps == 0) && ac+pc+as+ps+w > largest {
							largest = ac + pc + as + ps + w
						}
					}
				}
			}
		}
		if q[0].Expected != largest+1 {
			t.Fatalf("bad oracle %d", i)
		}
	}
}
func TestSVGBoundary(t *testing.T) {
	wrap := func(body string) string {
		return `<svg xmlns="http://www.w3.org/2000/svg" width="600" height="400" viewBox="0 0 600 400">` + body + `</svg>`
	}
	for _, bad := range []string{wrap(`<script>alert(1)</script>`), wrap(`<image href="file:///etc/passwd"/>`), wrap(`<rect style="fill:url(https://example.com)"/>`), wrap(`<g onload="x"/>`), `<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]>` + wrap(`&x;`), wrap(`<svg/>`), wrap(`<circle r="NaN"/>`), wrap(`<foreignObject/>`), wrap(`<path fill="url(#a)"/>`), wrap(`hello`)} {
		if _, _, err := ValidateSVG(bad, false); err == nil {
			t.Errorf("accepted unsafe input %s", bad)
		}
	}
	if _, _, err := ValidateSVG(wrap(`<path d="M0 0L10 10" fill="#fff"/>`), false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidateSVG(wrap(`<path d="M0 0L10 10"/>`), true); err == nil {
		t.Fatal("geometry accepted path")
	}
}
