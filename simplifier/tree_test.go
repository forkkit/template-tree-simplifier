package simplifier_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/mh-cbon/template-tree-simplifier/simplifier"
)

type TestData struct {
	tplstr         string
	data           interface{}
	expectTplStr   string
	funcs          template.FuncMap
	simplify       bool
	unshadow       bool
	typecheck      bool
	unhole         bool
	usedot         bool
	expectDotUse   bool
	printsanything bool
	expectPrints   bool
	checkedTypes   []map[string]reflect.Type
}

type tplData struct {
	Some string
}

func (t tplData) Method(s string) string {
	return s
}

func TestAll(t *testing.T) {
	//-
	defFuncs := template.FuncMap{
		"split": strings.Split,
		"join":  func(sep string, a []string) string { return strings.Join(a, sep) },
		"up":    strings.ToUpper,
		"lower": strings.ToLower,
		"incr":  func(s int) int { return s + 1 },
		"mul":   func(s int, d int) int { return s * d },
	}

	testTable := []TestData{
		TestData{
			tplstr:       `{{"son" | split "wat"}}`,
			expectTplStr: `{{$var0 := split "wat" "son"}}{{$var0}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{. | up}}`,
			expectTplStr: `{{$var0 := up .}}{{$var0}}`,
			funcs:        defFuncs,
			data:         "hello",
			simplify:     true,
		},
		TestData{
			tplstr:       `{{.S | up}}`,
			expectTplStr: `{{$var1 := .S}}{{$var0 := up $var1}}{{$var0}}`,
			funcs:        defFuncs,
			data:         struct{ S string }{S: "hello"},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{1 | incr}}`,
			expectTplStr: `{{$var0 := incr 1}}{{$var0}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{1 | incr | incr | mul 2}}`,
			expectTplStr: `{{$var1 := incr 1}}{{$var2 := incr $var1}}{{$var0 := mul 2 $var2}}{{$var0}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{mul (1 | incr) (2 | incr)}}`,
			expectTplStr: `{{$var0 := incr 1}}{{$var1 := incr 2}}{{$var2 := mul $var0 $var1}}{{$var2}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{"some" | split ("what" | up)}}`,
			expectTplStr: `{{$var0 := up "what"}}{{$var1 := split $var0 "some"}}{{$var1}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{"some" | split (.S | up)}}`,
			expectTplStr: `{{$var1 := .S}}{{$var0 := up $var1}}{{$var2 := split $var0 "some"}}{{$var2}}`,
			funcs:        defFuncs,
			data:         struct{ S string }{S: "hello"},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{"some" | split (("what" | lower) | up)}}`,
			expectTplStr: `{{$var1 := lower "what"}}{{$var0 := up $var1}}{{$var2 := split $var0 "some"}}{{$var2}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{"some" | split ((.S | lower) | up)}}`,
			expectTplStr: `{{$var2 := .S}}{{$var1 := lower $var2}}{{$var0 := up $var1}}{{$var3 := split $var0 "some"}}{{$var3}}`,
			funcs:        defFuncs,
			data:         struct{ S string }{S: "hello"},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{("what" | lower) | split "" | join "" | up}}`,
			expectTplStr: `{{$var2 := lower "what"}}{{$var1 := split "" $var2}}{{$var0 := join "" $var1}}{{$var3 := up $var0}}{{$var3}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{up "what" | lower}}`,
			expectTplStr: `{{$var1 := up "what"}}{{$var0 := lower $var1}}{{$var0}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{up .S | lower}}`,
			expectTplStr: `{{$var0 := .S}}{{$var2 := up $var0}}{{$var1 := lower $var2}}{{$var1}}`,
			funcs:        defFuncs,
			data:         struct{ S string }{S: "hello"},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$t := "what" | up}}`,
			expectTplStr: `{{$tplT := up "what"}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$t := ("what" | up)}}`,
			expectTplStr: `{{$tplT := up "what"}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$t := (.S | up)}}`,
			expectTplStr: `{{$var0 := .S}}{{$tplT := up $var0}}`,
			funcs:        defFuncs,
			data:         struct{ S string }{S: "hello"},
			simplify:     true,
		},
		TestData{
			tplstr: `{{$t := ("what" | up)}}
{{$k := up $t}}`,
			expectTplStr: `{{$tplT := up "what"}}
{{$tplK := up $tplT}}`,
			funcs:    defFuncs,
			simplify: true,
		},
		TestData{
			tplstr: `{{$t := (.S | up)}}
{{$k := up $t}}`,
			expectTplStr: `{{$var0 := .S}}{{$tplT := up $var0}}
{{$tplK := up $tplT}}`,
			funcs:    defFuncs,
			data:     struct{ S string }{S: "hello"},
			simplify: true,
		},
		TestData{
			tplstr:       `{{if true}}{{end}}`,
			expectTplStr: `{{if true}}{{end}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{if eq 1 2}}{{end}}`,
			expectTplStr: `{{$var0 := eq 1 2}}{{if $var0}}{{end}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr: `{{if true}}
  {{"some" | split ("what" | up)}}
{{end}}`,
			expectTplStr: `{{if true}}
  {{$var0 := up "what"}}{{$var1 := split $var0 "some"}}{{$var1}}
{{end}}`,
			funcs:    defFuncs,
			simplify: true,
		},
		TestData{
			tplstr: `{{if true}}
  {{"some" | split (.S | up)}}
{{end}}`,
			expectTplStr: `{{if true}}
  {{$var1 := .S}}{{$var0 := up $var1}}{{$var2 := split $var0 "some"}}{{$var2}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ S string }{S: "hello"},
			simplify: true,
		},
		TestData{
			tplstr:       `{{if eq ("what" | up | lower) "what"}}{{end}}`,
			expectTplStr: `{{$var2 := up "what"}}{{$var1 := lower $var2}}{{$var0 := eq $var1 "what"}}{{if $var0}}{{end}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{if eq (.S | up | lower) .S}}{{end}}`,
			expectTplStr: `{{$var2 := .S}}{{$var3 := up $var2}}{{$var1 := lower $var3}}{{$var4 := .S}}{{$var0 := eq $var1 $var4}}{{if $var0}}{{end}}`,
			funcs:        defFuncs,
			data:         struct{ S string }{S: "hello"},
			simplify:     true,
		},
		TestData{
			tplstr: `{{if eq ("what" | up | lower) "what"}}
  {{"some" | split ("what" | up)}}
{{end}}`,
			expectTplStr: `{{$var2 := up "what"}}{{$var1 := lower $var2}}{{$var0 := eq $var1 "what"}}{{if $var0}}
  {{$var3 := up "what"}}{{$var4 := split $var3 "some"}}{{$var4}}
{{end}}`,
			funcs:    defFuncs,
			simplify: true,
		},
		TestData{
			tplstr: `{{if eq (.S | up | lower) "what"}}
  {{.S | split ("what" | up)}}
{{end}}`,
			expectTplStr: `{{$var2 := .S}}{{$var3 := up $var2}}{{$var1 := lower $var3}}{{$var0 := eq $var1 "what"}}{{if $var0}}
  {{$var4 := up "what"}}{{$var6 := .S}}{{$var5 := split $var4 $var6}}{{$var5}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ S string }{S: "hello"},
			simplify: true,
		},
		TestData{
			tplstr: `{{if not (eq ("what" | up | lower) "what")}}
  {{"some" | split ("what" | up)}}
{{end}}`,
			expectTplStr: `{{$var3 := up "what"}}{{$var2 := lower $var3}}{{$var1 := eq $var2 "what"}}{{$var0 := not $var1}}{{if $var0}}
  {{$var4 := up "what"}}{{$var5 := split $var4 "some"}}{{$var5}}
{{end}}`,
			funcs:    defFuncs,
			simplify: true,
		},
		TestData{
			tplstr: `{{if not (eq (.S | up | lower) "what")}}
  {{"some" | split (.S | up)}}
{{end}}`,
			expectTplStr: `{{$var3 := .S}}{{$var4 := up $var3}}{{$var2 := lower $var4}}{{$var1 := eq $var2 "what"}}{{$var0 := not $var1}}{{if $var0}}
  {{$var6 := .S}}{{$var5 := up $var6}}{{$var7 := split $var5 "some"}}{{$var7}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ S string }{S: "hello"},
			simplify: true,
		},
		TestData{
			tplstr:       `{{$var0 := up "what" | lower}}{{$var0}}`,
			expectTplStr: `{{$var0 := up "what"}}{{$tplVar0 := lower $var0}}{{$tplVar0}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$var0 := up .S | lower}}{{$var0}}`,
			expectTplStr: `{{$var0 := .S}}{{$var1 := up $var0}}{{$tplVar0 := lower $var1}}{{$tplVar0}}`,
			funcs:        defFuncs,
			data:         struct{ S string }{S: "hello"},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{if not (eq "a" "b")}}not eq{{end}}`,
			expectTplStr: `{{$var1 := eq "a" "b"}}{{$var0 := not $var1}}{{if $var0}}not eq{{end}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{if eq ("what" |lower) ("what" |lower)}}{{end}}`,
			expectTplStr: `{{$var1 := lower "what"}}{{$var2 := lower "what"}}{{$var0 := eq $var1 $var2}}{{if $var0}}{{end}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$var0 := eq (lower "up") "what"}}`,
			expectTplStr: `{{$var0 := lower "up"}}{{$tplVar0 := eq $var0 "what"}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr: `{{range .List}}
{{.}}
{{end}}`,
			expectTplStr: `{{$var0 := .List}}{{range $var0}}
{{.}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ List []string }{List: []string{"what"}},
			simplify: true,
		},
		TestData{
			tplstr: `{{range split "what" ""}}
{{.}}
{{end}}`,
			expectTplStr: `{{$var0 := split "what" ""}}{{range $var0}}
{{.}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ List []string }{List: []string{"what"}},
			simplify: true,
		},
		TestData{
			tplstr: `{{range $i,$v := split "what" ""}}
{{$i}} -> {{$v}}
{{end}}`,
			expectTplStr: `{{$var0 := split "what" ""}}{{range $tplI, $tplV := $var0}}
{{$tplI}} -> {{$tplV}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ List []string }{List: []string{"what"}},
			simplify: true,
		},
		TestData{
			tplstr: `{{range $i,$v := "some" | split (("what" | lower) | up)}}
{{$i}} -> {{$v}}
{{end}}`,
			expectTplStr: `{{$var2 := lower "what"}}{{$var1 := up $var2}}{{$var0 := split $var1 "some"}}{{range $tplI, $tplV := $var0}}
{{$tplI}} -> {{$tplV}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ List []string }{List: []string{"what"}},
			simplify: true,
		},
		TestData{
			tplstr: `{{if false}}
{{"some" | split (("what" | lower) | up)}}
{{else}}
{{"some" | split (("what" | lower) | up)}}
{{end}}`,
			expectTplStr: `{{if false}}
{{$var1 := lower "what"}}{{$var0 := up $var1}}{{$var2 := split $var0 "some"}}{{$var2}}
{{else}}
{{$var4 := lower "what"}}{{$var3 := up $var4}}{{$var5 := split $var3 "some"}}{{$var5}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ List []string }{List: []string{"what"}},
			simplify: true,
		},
		TestData{
			tplstr: `{{if false}}
  {{"some" | split (("what" | lower) | up)}}
{{else}}
  {{"some" | split (("what" | lower) | up)}}
  {{if false}}
    {{"some" | split (("what" | lower) | up)}}
  {{else}}
    {{"some" | split (("what" | lower) | up)}}
  {{end}}
{{end}}`,
			expectTplStr: `{{if false}}
  {{$var1 := lower "what"}}{{$var0 := up $var1}}{{$var2 := split $var0 "some"}}{{$var2}}
{{else}}
  {{$var4 := lower "what"}}{{$var3 := up $var4}}{{$var5 := split $var3 "some"}}{{$var5}}
  {{if false}}
    {{$var7 := lower "what"}}{{$var6 := up $var7}}{{$var8 := split $var6 "some"}}{{$var8}}
  {{else}}
    {{$var10 := lower "what"}}{{$var9 := up $var10}}{{$var11 := split $var9 "some"}}{{$var11}}
  {{end}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ List []string }{List: []string{"what"}},
			simplify: true,
		},
		TestData{
			tplstr: `{{range .List}}
  This is the range branch
  {{"some" | split (("what" | lower) | up)}}
{{else}}
  This is the else branch
  {{"some" | split (("what" | lower) | up)}}
{{end}}`,
			expectTplStr: `{{$var0 := .List}}{{range $var0}}
  This is the range branch
  {{$var2 := lower "what"}}{{$var1 := up $var2}}{{$var3 := split $var1 "some"}}{{$var3}}
{{else}}
  This is the else branch
  {{$var5 := lower "what"}}{{$var4 := up $var5}}{{$var6 := split $var4 "some"}}{{$var6}}
{{end}}`,
			funcs:    defFuncs,
			data:     struct{ List []string }{List: []string{"what"}},
			simplify: true,
		},
		TestData{
			tplstr:       `{{with $x := "output"}}{{. | up}}{{end}}`,
			expectTplStr: `{{with $tplX := "output"}}{{$var0 := up .}}{{$var0}}{{end}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{with $x := "output"}}{{$ | up}}{{end}}`,
			expectTplStr: `{{with $tplX := "output"}}{{$var0 := up $}}{{$var0}}{{end}}`,
			funcs:        defFuncs,
			data:         "hello",
			simplify:     true,
		},
		TestData{
			tplstr:       `{{with $x := "output" | lower}}{{$ | up}}{{. | up}}{{end}}`,
			expectTplStr: `{{$var0 := lower "output"}}{{with $tplX := $var0}}{{$var1 := up $}}{{$var1}}{{$var2 := up .}}{{$var2}}{{end}}`,
			funcs:        defFuncs,
			data:         "hello",
			simplify:     true,
		},
		TestData{
			tplstr:       `{{with $x := "output" | lower}}{{$ | up}}{{. | up | lower}}{{end}}`,
			expectTplStr: `{{$var0 := lower "output"}}{{with $tplX := $var0}}{{$var1 := up $}}{{$var1}}{{$var3 := up .}}{{$var2 := lower $var3}}{{$var2}}{{end}}`,
			funcs:        defFuncs,
			data:         "hello",
			simplify:     true,
		},
		TestData{
			tplstr:       `{{with $x := "" | lower}}{{else}}{{$ | up}}{{. | up | lower}}{{end}}`,
			expectTplStr: `{{$var0 := lower ""}}{{with $tplX := $var0}}{{else}}{{$var1 := up $}}{{$var1}}{{$var3 := up .}}{{$var2 := lower $var3}}{{$var2}}{{end}}`,
			funcs:        defFuncs,
			data:         "hello",
			simplify:     true,
		},
		TestData{
			tplstr:       `{{.S.S}}`,
			expectTplStr: `{{$var0 := .S.S}}{{$var0}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S string } }{S: struct{ S string }{S: "hello"}},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{split "" .S.S}}`,
			expectTplStr: `{{$var0 := .S.S}}{{$var1 := split "" $var0}}{{$var1}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S string } }{S: struct{ S string }{S: "hello"}},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{call .S}}`,
			expectTplStr: `{{$var0 := .S}}{{$var1 := call $var0}}{{$var1}}`,
			funcs:        defFuncs,
			data:         struct{ S func() string }{S: func() string { return "hello" }},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{if .S.S}}this is true{{end}}`,
			expectTplStr: `{{$var0 := .S.S}}{{if $var0}}this is true{{end}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S bool } }{S: struct{ S bool }{S: true}},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{with $y := .S.S}}plop{{end}}`,
			expectTplStr: `{{$var0 := .S.S}}{{with $tplY := $var0}}plop{{end}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S bool } }{S: struct{ S bool }{S: true}},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{range .S.S}}{{.}}{{end}}`,
			expectTplStr: `{{$var0 := .S.S}}{{range $var0}}{{.}}{{end}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S []string } }{S: struct{ S []string }{S: []string{"hello"}}},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$x := .}}{{$x.S.S}}`,
			expectTplStr: `{{$tplX := .}}{{$var0 := $tplX.S.S}}{{$var0}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S []string } }{S: struct{ S []string }{S: []string{"hello"}}},
			simplify:     true,
		},
		TestData{
			tplstr:       `some{{define "rr"}}what{{end}}ww{{template "rr"}}`,
			expectTplStr: `someww{{template "rr"}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S []string } }{S: struct{ S []string }{S: []string{"hello"}}},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{define "rr"}}what{{end}}ww{{template "rr" .S.S}}`,
			expectTplStr: `ww{{$var0 := .S.S}}{{template "rr" $var0}}`,
			funcs:        defFuncs,
			data:         struct{ S struct{ S []string } }{S: struct{ S []string }{S: []string{"hello"}}},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{define "rr"}}what{{end}}ww{{template "rr" (up "rr")}}`,
			expectTplStr: `ww{{$var0 := up "rr"}}{{template "rr" $var0}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$x := "r"}}{{(((($x))))}}`,
			expectTplStr: `{{$tplX := "r"}}{{$tplX}}`,
			funcs:        defFuncs,
			simplify:     true,
		},
		TestData{
			tplstr:       `{{.Method "ff"}}`,
			expectTplStr: `{{$var0 := .Method "ff"}}{{$var0}}`,
			funcs:        defFuncs,
			data:         tplData{},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{.Method "ff" | up}}`,
			expectTplStr: `{{$var0 := .Method "ff"}}{{$var1 := up $var0}}{{$var1}}`,
			funcs:        defFuncs,
			data:         tplData{},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$x := .}}{{$x.Method "ff"}}`,
			expectTplStr: `{{$tplX := .}}{{$var0 := $tplX.Method "ff"}}{{$var0}}`,
			funcs:        defFuncs,
			data:         tplData{},
			simplify:     true,
		},
		TestData{
			tplstr:       `{{$x := .}}{{$x.Method "ff" | up}}`,
			expectTplStr: `{{$tplX := .}}{{$var0 := $tplX.Method "ff"}}{{$var1 := up $var0}}{{$var1}}`,
			funcs:        defFuncs,
			data:         tplData{},
			simplify:     true,
		},
	}

	for i, testData := range testTable {
		if execTestData(testData, t, i) == false {
			break
		}
	}
}

func execTestData(testData TestData, t *testing.T, index int) bool {
	// ensure the template is valid and working
	tpl, err := template.New("").Funcs(testData.funcs).Parse(testData.tplstr)
	if err != nil {
		t.Logf("ORIGINAL TEMPLATE:\n%v\n", testData.tplstr)
		t.Errorf("Test(%v): Failed to compiling original template: %v", index, err)
		return false
	}
	// execute template, check everything is still fine
	originalOut, err := exectemplate(tpl, testData.data)
	if err != nil {
		t.Logf("ORIGINAL TEMPLATE:\n%v\n", testData.tplstr)
		t.Errorf("Test(%v): Failed to execute original template: %v", index, err)
		return false
	}
	var modifiedTemplate *template.Template
	var typeCheck *simplifier.State
	if testData.simplify {
		// do the simplification
		modifiedTemplate = simplifytemplate(tpl)
	} else if testData.unshadow {
		// do the unshadowing
		modifiedTemplate = unshadowtemplate(tpl)
	} else if testData.typecheck {
		// do the unholing
		modifiedTemplate, typeCheck = typechecktemplate(tpl, testData)
	} else if testData.unhole {
		// do the unholing
		modifiedTemplate, typeCheck = unholetemplate(tpl, testData)
		// fmt.Printf("%#v\n", modifiedTemplate.Tree.Root.String())
	} else if testData.usedot {
		// do the usedot test
		usingDot := usedottemplate(tpl)
		if usingDot != testData.expectDotUse {
			t.Errorf(
				"Test(%v): UseDot did not match expected=%v, got=%v\nORIGINAL\n%v\n",
				index,
				testData.expectDotUse,
				usingDot,
				testData.tplstr)
			return false
		}
		return true
	} else if testData.printsanything {
		// do the PrintsAnything test
		isPrinting := printsanythingtemplate(tpl)
		if isPrinting != testData.expectPrints {
			t.Errorf(
				"Test(%v): PrintsAnything expectation did not match, expected=%v, got=%v\nORIGINAL\n%v\n",
				index,
				testData.expectPrints,
				isPrinting,
				testData.tplstr)
			return false
		}
		return true
	}
	// execute simplified template, check everything is still fine
	simplifiedOut, err := exectemplate(modifiedTemplate, testData.data)
	if err != nil {
		t.Logf("ORIGINAL TEMPLATE:\n%v\n", testData.tplstr)
		t.Logf("SIMPLIFIED TEMPLATE\n%v\n", modifiedTemplate.Tree.Root.String())
		t.Errorf("Test(%v): Failed to execute the simplified template: %v", index, err)
		return false
	}
	// ensure both output are eq
	if originalOut != simplifiedOut {
		t.Errorf("Test(%v): Unexpected template outputs\nORIGINAL\n%v\nSIMPLIFIED\n%v\n",
			index, originalOut, simplifiedOut)
		return false
	}
	// ensure the new template matches expected simplified template
	simplified := fmt.Sprintf("%v", modifiedTemplate.Tree.Root.String())
	if simplified != testData.expectTplStr {
		t.Errorf("Test(%v): Unexpected simplified template content\nEXPECTED\n%v\nSIMPLIFIED\n%v\n",
			index, testData.expectTplStr, simplified)
		return false
	}
	if typeCheck != nil {
		if len(testData.checkedTypes) != typeCheck.Len() {
			t.Errorf("Test(%v): Unexpected typechecker number of scopes, expected=%v, got=%v\nTEMPLATE: %v",
				index, len(testData.checkedTypes), typeCheck.Len(), testData.tplstr)
			return false
		}
		for i, scope := range testData.checkedTypes {
			typeCheck.Enter()
			for vard, typed := range scope {
				if typeCheck.HasVar(vard) == false {
					t.Errorf("Test(%v): Expected type checker scope(%v) to contain the variable=%v\nTEMPLATE:%v\nSIMPLIFIED:\n%v\n%#v",
						index, i, vard, testData.tplstr, simplified, typeCheck)
					return false
				} else {
					if typed != typeCheck.GetVar(vard) {
						t.Errorf("Test(%v): Expected scope(%v) to contain the variable=%v with the same reflect.Type, expected=%v, got=%v\nTEMPLATE:%v",
							index, i, vard, typed, typeCheck.GetVar(vard), testData.tplstr)
						return false
					}
				}
			}
			for vard, _ := range typeCheck.Current() {
				if _, ok := scope[vard]; ok == false {
					t.Errorf("Test(%v): Uexpected variable=%v in scope(%v)\nTEMPLATE:%v",
						index, vard, i, testData.tplstr)
					return false
				}
			}
		}
	}
	return true
}

func simplifytemplate(t *template.Template) *template.Template {
	ret, err := t.Clone()
	if err != nil {
		panic(err)
	}
	for _, t := range ret.Templates() {
		if t.Tree != nil {
			simplifier.Simplify(t.Tree)
		}
	}
	return ret
}

func exectemplate(t *template.Template, data interface{}) (string, error) {
	var b bytes.Buffer
	return b.String(), t.Execute(&b, data)
}
